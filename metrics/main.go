package main

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

var gMetrics []string

func headers(w http.ResponseWriter, req *http.Request) {

	// This handler does something a little more
	// sophisticated by reading all the HTTP request
	// headers and echoing them into the response body.
	fmt.Println("Metric request from", req.RemoteAddr)
	for _, mString := range gMetrics {
		fmt.Fprintf(w, "%v\n", mString)
	}
}

func getMetrics() {
	for {
		var metricData []string
		mNameWash, err := regexp.Compile("[^a-zA-Z0-9]+")
		mValueWash, err := regexp.Compile("[^0-9.]+")

		time.Sleep(5 * time.Second)
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
			containerData[0] = mNameWash.ReplaceAllString(containerData[0], "_")
			containerData[1] = mValueWash.ReplaceAllString(containerData[1], "")
			containerData[2] = mValueWash.ReplaceAllString(containerData[2], "")
			containerData[3] = mValueWash.ReplaceAllString(containerData[3], "")

			metricData = append(metricData, "# HELP br_"+containerData[0]+"_cpu_p Current CPU usage")
			metricData = append(metricData, "# type br_"+containerData[0]+"_cpu_p gauge")
			metricData = append(metricData, "br_"+containerData[0]+"_cpu_p "+containerData[1])

			metricData = append(metricData, "# HELP br_"+containerData[0]+"_mem_p Current MEM usage")
			metricData = append(metricData, "# type br_"+containerData[0]+"_mem_p gauge")
			metricData = append(metricData, "br_"+containerData[0]+"_mem_p "+containerData[2])

			metricData = append(metricData, "# HELP br_"+containerData[0]+"_pidsCount Pids count")
			metricData = append(metricData, "# type br_"+containerData[0]+"_pidsCount gauge")
			metricData = append(metricData, "br_"+containerData[0]+"_pidsCount "+containerData[3])

		}
		gMetrics = metricData
	}
}

func main() {
	go getMetrics()
	fmt.Println("start")
	http.HandleFunc("/metrics", headers)
	log.Fatal(http.ListenAndServe("10.138.106.86:8080", nil))
}
