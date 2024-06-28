package main

import (
	"fmt"
	"time"

	"github.com/oarkflow/expr"
)

var data = map[string]interface{}{
	"name": "Sujit Baniya",
	"address": map[string]any{
		"city": "Kathmandu",
	},
	"gender": "male",
	"company": map[string]any{
		"name": "Orgware Construct Pvt. Ltd",
		"A":    1,
		"B":    5,
	},
	"position":   "Associate Developer",
	"start_date": "2021-09-01",
	"end_date":   "2022-09-30",
}

func main() {
	expr.AddFunction("current_date", func(params ...any) (any, error) {
		return time.Now().Format(time.DateOnly), nil
	})

	start := time.Now()
	p, err := expr.Eval("4+5", data)
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	fmt.Println(expr.AvailableFunctions())
	fmt.Println(fmt.Sprintf("%s", time.Since(start)))
}
