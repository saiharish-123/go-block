package main

import (
	"fmt"
)

var man = "man"

func helloworld() string {
	fmt.Println("Hello World!")
	// var nuvu2 int = 123
	var nuvu float32 = 123
	fmt.Println(man)
	man = "asd"
	fmt.Printf("dass %T\n", nuvu)
	fmt.Println(man)
	var i int = 16
	var j float32 = 16
	fmt.Printf("%6.2f\n", j)
	fmt.Printf("%#X", i)
	fmt.Printf("\n")
	fmt.Printf("%#x", i)
	var fatam = [...]uint64{0: 1, 2: 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	numbers := fatam[0:11]
	fmt.Println(len(numbers))
	numbers = append(numbers, 12, 13, 14, 15)
	fmt.Println(cap(numbers))
	fmt.Print(numbers)
	var cars = [4]string{"Volvo", "BMW", "Ford", "Mazda"}
	fmt.Print(cars)
	fmt.Printf("\n")
	return "asdsd"
}
