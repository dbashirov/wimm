package test

import "fmt"

func Echo(ch string) {
	// fmt.Println(<-ch)
	// fmt.Println(<-ch)
	fmt.Println(ch)
}

func main() {

	ch := make(chan string, 1)

	// go echo(ch)

	ch <- "Hello world"
	// fmt.Println("test")
	fmt.Println(<-ch)
	ch <- "Hello1"
	fmt.Println(<-ch)
	fmt.Println("Stoped program")
}
