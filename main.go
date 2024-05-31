package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
)

func main() {
	response, err := http.Get("https://api.ipify.org")
	if err != nil {
		fmt.Println("Error fetching IP: ", err)
		return
	}
	defer response.Body.Close()

	ip, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Error reading response: ", err)
		return
	}

	fmt.Println("External IP address:", string(ip))
}
