package main

import (
    "fmt"
    "regexp"
)

func main () {
    rtest := regexp.MustCompile("[[:space:]]{2,}")
    tststring := "test:     1 2 3"
    outstr := rtest.ReplaceAllString(tststring," ")
    fmt.Println(outstr)
}