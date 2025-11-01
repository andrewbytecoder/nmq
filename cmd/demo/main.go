package main

import "fmt"

func main() {

	var data map[string]string

	data = map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	for k := range data {
		fmt.Println(k)
	}

}
