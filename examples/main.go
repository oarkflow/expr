package main

import (
	"encoding/json"
	"fmt"
	"os"
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

type Test struct {
	Pattern string `json:"pattern"`
}

func main() {
	expr.AddFunction("current_date", func(params ...any) (any, error) {
		return time.Now().Format(time.DateOnly), nil
	})
	var test Test
	bt, _ := os.ReadFile("test.json")
	json.Unmarshal(bt, &test)
	start := time.Now()
	p, err := expr.Eval(test.Pattern, data)
	if err != nil {
		panic(err)
	}
	fmt.Println(p)
	fmt.Println(fmt.Sprintf("%s", time.Since(start)))
}
