package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	//resp, err := http.Get("http://10.30.50.171/pfcs/v1/reflushCache")
	//respBody, _ := ioutil.ReadAll(resp.Body)
	//fmt.Printf("%s %v", respBody, err)
	resp, err := http.Post("http://10.30.48.54:80/reports/select/LineNo", "application/json", bytes.NewBufferString(""))
	respBody, _ := ioutil.ReadAll(resp.Body)
	fmt.Printf("%s %v", respBody, err)
}
