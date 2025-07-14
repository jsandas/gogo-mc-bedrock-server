package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"time"

	lorem "github.com/drhodes/golorem"
)

func main() {

	input := make(chan string)
	go func(in chan string) {
		reader := bufio.NewReader(os.Stdin)
		for {
			// read by one line (enter pressed)
			s, err := reader.ReadString('\n')
			// check for errors
			if err != nil {
				// close channel just to inform others
				close(in)
				log.Println("Error in read string", err)
			}
			in <- s
		}
	}(input)

	go func(in chan string) {
		for {
			in <- "test-app " + lorem.Word(4, 10)
			n := rand.Intn(10)
			time.Sleep(time.Duration(n) * time.Second)
		}
	}(input)

	for {
		in := <-input
		// remove all leading and trailing white space
		in = strings.TrimSpace(in)
		// do what you want with input data
		fmt.Println("Read from stdin: ", in)
	}
}
