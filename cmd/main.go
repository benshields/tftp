package main

import (
	"fmt"
	"github.com/benshields/tftp"
)

func main() {
	srv := tftp.NewServer("C:/Users/ben/Desktop", ":tftp", nil)
	stop := make(chan tftp.CancelType)
	done := srv.Serve(stop)
	fmt.Println(<-done)
}
