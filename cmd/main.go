package main

import (
	"fmt"
	"os"
)

func main() {
	//srv := tftp.NewServer("C:/Users/ben/Desktop", ":tftp", nil)
	//stop := make(chan tftp.CancelType)
	//done := srv.Serve(stop)
	//fmt.Println(<-done)
	filename := "C:/Users/ben/Desktop/put3.txt"
	_, err := os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY|os.O_EXCL, os.ModePerm)
	fmt.Println(err)
	fmt.Println(os.IsExist(err))
}
