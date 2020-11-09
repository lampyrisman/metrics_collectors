package main

import (
	"fmt"
	"log"
	"net/http"
	"io/ioutil"
	"strings"
	"os"
	"os/exec"
	"strconv"
	"time"
	"regexp"
	syscall "golang.org/x/sys/unix"
)

const (
    B  = 1
    KB = 1024 * B
    MB = 1024 * KB
    GB = 1024 * MB
)

var hostName string
var gMetrics []string
func headers(w http.ResponseWriter, req *http.Request) {

	// This handler does something a little more
	// sophisticated by reading all the HTTP request
	// headers and echoing them into the response body.
	fmt.Println("Metric request from", req.RemoteAddr)
	for _, mString := range gMetrics {
		fmt.Fprintf(w, "%v\n",  mString)
	}
}

func getProcFile (filename string) (outlines []string){
    r_line := regexp.MustCompile("[[:space:]]{2,}")
    content, err := ioutil.ReadFile("/proc/"+filename)
    if err != nil {
        log.Fatal("ERROR!", err)
    }

    cleanstr := r_line.ReplaceAllString(string(content)," ")
    outlines = strings.Split(cleanstr, "\n")
    return outlines
}


func getCPUSample() (idle, total uint64) {
    lines := getProcFile("stat")
    for _, line := range(lines) {
        fields := strings.Fields(line)
        if fields[0] == "cpu" {
            numFields := len(fields)
            for i := 1; i < numFields; i++ {
                val, err := strconv.ParseUint(fields[i], 10, 64)
                if err != nil {
                    fmt.Println("Error: ", i, fields[i], err)
                }
                total += val // tally up all the numbers to get total ticks
                if i == 4 {  // idle is the 5th field in the cpu line
                    idle = val
                }
            }
            return
        }
    }
    return
}

func getMemUsagePercent()(outdata string){

    var memTotal,memFree float64
    lines := getProcFile("meminfo")

    for _, line := range(lines){

	fields := strings.Split(line," ")
	if fields[0] == "MemTotal:" { memTotal, _ = strconv.ParseFloat(fields[1], 64)}
	if fields[0] == "MemFree:" { memFree, _ = strconv.ParseFloat(fields[1], 64)}
    }
	outdata =  fmt.Sprintf("%f", 100 * (memTotal - memFree) / memTotal)
    return 
}


func getCpuUsagePercent()(outdata string) {
    idle0, total0 := getCPUSample()
    time.Sleep(1 * time.Second)
    idle1, total1 := getCPUSample()
    idleTicks := float64(idle1 - idle0)
    totalTicks := float64(total1 - total0)
    outdata = fmt.Sprintf("%f",100 * (totalTicks - idleTicks) / totalTicks)
    return
}

func getDiskUsagePercent(path string) (outdata string){
    fs := syscall.Statfs_t{}
    err := syscall.Statfs(path, &fs)
    if err != nil {
	return
    }
//    disk.All = fs.Blocks * float64(fs.Bsize)
//    disk.Avail = fs.Bavail * float64(fs.Bsize)
//    disk.Free = fs.Bfree * float64(fs.Bsize)
    outdata = fmt.Sprintf("%f",100 * float64(fs.Blocks * uint64(fs.Bsize) - fs.Bfree * uint64(fs.Bsize)) / float64(fs.Blocks * uint64(fs.Bsize)))
    return 
}

func getDisks ()(mounts []string) {
    r_disk := regexp.MustCompile("^/dev/")
    lines := getProcFile("mounts")
    for _, line := range lines {
	if r_disk.MatchString(line) == true {
	    fields := strings.Split(line," ")
	    mounts = append(mounts,fields[1])
	}
    }
return
}

func getDockerMetrics()(metricData []string) {
		mNameWash, err := regexp.Compile("[^a-zA-Z0-9]+")
	mValueWash, err := regexp.Compile("[^0-9.]+")
	out, err := exec.Command("docker", "stats", "--no-stream", "--format={{.Name}} {{.CPUPerc}} {{.MemPerc}} {{.PIDs}}").Output()
	if err != nil {
	    panic(err)
	}
	containers := strings.Split(string(out), "\n")
	for _, contStat := range containers {
	    if contStat == "" {
		continue
	    }
	    containerData := strings.Split(contStat, " ")
	    containerData[0] = mNameWash.ReplaceAllString(containerData[0], "")
	    containerData[1] = mValueWash.ReplaceAllString(containerData[1], "")
	    containerData[2] = mValueWash.ReplaceAllString(containerData[2], "")
	    containerData[3] = mValueWash.ReplaceAllString(containerData[3], "")

	    metricData = append(metricData, "# HELP "+hostName+"_"+containerData[0]+"_cpu_p Current CPU usage")
	    metricData = append(metricData, "# type "+hostName+"_"+containerData[0]+"_cpu_p gauge")
	    metricData = append(metricData, hostName+"_"+containerData[0]+"_cpu_p "+containerData[1])

	    metricData = append(metricData, "# HELP "+hostName+"_"+containerData[0]+"_mem_p Current MEM usage")
	    metricData = append(metricData, "# type "+hostName+"_"+containerData[0]+"_mem_p gauge")
	    metricData = append(metricData, hostName+"_"+containerData[0]+"_mem_p "+containerData[2])

	    metricData = append(metricData, "# HELP "+hostName+"_"+containerData[0]+"_pidsCount Pids count")
	    metricData = append(metricData, "# type "+hostName+"_"+containerData[0]+"_pidsCount gauge")
	    metricData = append(metricData, hostName+"_"+containerData[0]+"_pidsCount "+containerData[3])

	}
	return
    }



func makeMetric () {
	var metricData []string

	metricData = append(metricData, "# HELP "+hostName+"_CPUpercent CPU Usage %")
	metricData = append(metricData, "# TYPE "+hostName+"_CPUpercent gauge")
	metricData = append(metricData, hostName+"_CPUpercent "+getCpuUsagePercent())

	metricData = append(metricData, "# HELP "+hostName+"_MEMpercent MEM Usage %")
	metricData = append(metricData, "# TYPE "+hostName+"_MEMpercent gauge")
	metricData = append(metricData, hostName+"_MEMpercent "+getMemUsagePercent())

	metricData = append(metricData, "# HELP "+hostName+"_DISKpercent DISK Usage %")
	metricData = append(metricData, "# TYPE "+hostName+"_DISKpercent gauge")
	for _, mount := range getDisks(){
	    metricData = append(metricData, hostName+`_DISKpercent{partition="`+mount+`"}`+getDiskUsagePercent(mount))
	}
	metricData = append(metricData, getDockerMetrics()...)
	gMetrics = metricData
}

func main() {
	if len(os.Args) != 3 {
	    log.Fatal("Usage: metrics <IP:port> <metric,metric,metric...> \n Available metrics: CpuMemDisk docker_CpuMem")
	}
	hostName, _ = os.Hostname()
	mNameWash, _ := regexp.Compile("[^a-zA-Z0-9]+")
	hostName = mNameWash.ReplaceAllString(hostName, "")

//	getCpuUsagePercent()
//	getMemUsagePercent()
//	getDisks()
	makeMetric()


	fmt.Println("start")
	http.HandleFunc("/metrics", headers)
	log.Fatal(http.ListenAndServe(os.Args[1], nil))
}
