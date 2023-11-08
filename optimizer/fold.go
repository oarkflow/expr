package optimizer

import (
	"fmt"
	"math"
	"reflect"

	"github.com/oarkflow/expr/ast"
	"github.com/oarkflow/expr/file"
)

var (
	integerType = reflect.TypeOf(0)
	floatType   = reflect.TypeOf(float64(0))
	stringType  = reflect.TypeOf("")
)

type fold struct {
	applied bool
	err     *file.Error
}

func (fold *fold) Visit(node *ast.Node) {
	patch := func(newNode ast.Node) {
		fold.applied = true
		ast.Patch(node, newNode)
	}
	patchWithType := func(newNode ast.Node) {
		patch(newNode)
		switch newNode.(type) {
		case *ast.IntegerNode:
			newNode.SetType(integerType)
		case *ast.FloatNode:
			newNode.SetType(floatType)
		case *ast.StringNode:
			newNode.SetType(stringType)
		default:
			panic(fmt.Sprintf("unknown type %T", newNode))
		}
	}

	switch n := (*node).(type) {
	case *ast.UnaryNode:
		switch n.Operator {
		case "-":
			if i, ok := n.Node.(*ast.IntegerNode); ok {
				patchWithType(&ast.IntegerNode{Value: -i.Value})
			}
			if i, ok := n.Node.(*ast.FloatNode); ok {
				patchWithType(&ast.FloatNode{Value: -i.Value})
			}
		case "+":
			if i, ok := n.Node.(*ast.IntegerNode); ok {
				patchWithType(&ast.IntegerNode{Value: i.Value})
			}
			if i, ok := n.Node.(*ast.FloatNode); ok {
				patchWithType(&ast.FloatNode{Value: i.Value})
			}
		case "!", "not":
			if a := toBool(n.Node); a != nil {
				patch(&ast.BoolNode{Value: !a.Value})
			}
		}

	case *ast.BinaryNode:
		switch n.Operator {
		case "+":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.IntegerNode{Value: a.Value + b.Value})
				}
			}
			{
				a := toInteger(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: float64(a.Value) + b.Value})
				}
			}
			{
				a := toFloat(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value + float64(b.Value)})
				}
			}
			{
				a := toFloat(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value + b.Value})
				}
			}
			{
				a := toString(n.Left)
				b := toString(n.Right)
				if a != nil && b != nil {
					patch(&ast.StringNode{Value: a.Value + b.Value})
				}
			}
		case "-":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.IntegerNode{Value: a.Value - b.Value})
				}
			}
			{
				a := toInteger(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: float64(a.Value) - b.Value})
				}
			}
			{
				a := toFloat(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value - float64(b.Value)})
				}
			}
			{
				a := toFloat(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value - b.Value})
				}
			}
		case "*":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.IntegerNode{Value: a.Value * b.Value})
				}
			}
			{
				a := toInteger(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: float64(a.Value) * b.Value})
				}
			}
			{
				a := toFloat(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value * float64(b.Value)})
				}
			}
			{
				a := toFloat(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value * b.Value})
				}
			}
		case "/":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: float64(a.Value) / float64(b.Value)})
				}
			}
			{
				a := toInteger(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: float64(a.Value) / b.Value})
				}
			}
			{
				a := toFloat(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value / float64(b.Value)})
				}
			}
			{
				a := toFloat(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: a.Value / b.Value})
				}
			}
		case "%":
			if a, ok := n.Left.(*ast.IntegerNode); ok {
				if b, ok := n.Right.(*ast.IntegerNode); ok {
					if b.Value == 0 {
						fold.err = &file.Error{
							Location: (*node).Location(),
							Message:  "integer divide by zero",
						}
						return
					}
					patch(&ast.IntegerNode{Value: a.Value % b.Value})
				}
			}
		case "**", "^":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: math.Pow(float64(a.Value), float64(b.Value))})
				}
			}
			{
				a := toInteger(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: math.Pow(float64(a.Value), b.Value)})
				}
			}
			{
				a := toFloat(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: math.Pow(a.Value, float64(b.Value))})
				}
			}
			{
				a := toFloat(n.Left)
				b := toFloat(n.Right)
				if a != nil && b != nil {
					patchWithType(&ast.FloatNode{Value: math.Pow(a.Value, b.Value)})
				}
			}
		case "and", "&&":
			a := toBool(n.Left)
			b := toBool(n.Right)

			if a != nil && a.Value { // true and x
				patch(n.Right)
			} else if b != nil && b.Value { // x and true
				patch(n.Left)
			} else if (a != nil && !a.Value) || (b != nil && !b.Value) { // "x and false" or "false and x"
				patch(&ast.BoolNode{Value: false})
			}
		case "or", "||":
			a := toBool(n.Left)
			b := toBool(n.Right)

			if a != nil && !a.Value { // false or x
				patch(n.Right)
			} else if b != nil && !b.Value { // x or false
				patch(n.Left)
			} else if (a != nil && a.Value) || (b != nil && b.Value) { // "x or true" or "true or x"
				patch(&ast.BoolNode{Value: true})
			}
		case "==":
			{
				a := toInteger(n.Left)
				b := toInteger(n.Right)
				if a != nil && b != nil {
					patch(&ast.BoolNode{Value: a.Value == b.Value})
				}
			}
			{
				a := toString(n.Left)
				b := toString(n.Right)
				if a != nil && b != nil {
					patch(&ast.BoolNode{Value: a.Value == b.Value})
				}
			}
			{
				a := toBool(n.Left)
				b := toBool(n.Right)
				if a != nil && b != nil {
					patch(&ast.BoolNode{Value: a.Value == b.Value})
				}
			}
		}

	case *ast.ArrayNode:
		if len(n.Nodes) > 0 {
			for _, a := range n.Nodes {
				switch a.(type) {
				case *ast.IntegerNode, *ast.FloatNode, *ast.StringNode, *ast.BoolNode:
					continue
				default:
					return
				}
			}
			value := make([]any, len(n.Nodes))
			for i, a := range n.Nodes {
				switch b := a.(type) {
				case *ast.IntegerNode:
					value[i] = b.Value
				case *ast.FloatNode:
					value[i] = b.Value
				case *ast.StringNode:
					value[i] = b.Value
				case *ast.BoolNode:
					value[i] = b.Value
				}
			}
			patch(&ast.ConstantNode{Value: value})
		}

	case *ast.BuiltinNode:
		switch n.Name {
		case "filter":
			if len(n.Arguments) != 2 {
				return
			}
			if base, ok := n.Arguments[0].(*ast.BuiltinNode); ok && base.Name == "filter" {
				patch(&ast.BuiltinNode{
					Name: "filter",
					Arguments: []ast.Node{
						base.Arguments[0],
						&ast.BinaryNode{
							Operator: "&&",
							Left:     base.Arguments[1],
							Right:    n.Arguments[1],
						},
					},
				})
			}
		}
	}
}

func toString(n ast.Node) *ast.StringNode {
	switch a := n.(type) {
	case *ast.StringNode:
		return a
	}
	return nil
}

func toInteger(n ast.Node) *ast.IntegerNode {
	switch a := n.(type) {
	case *ast.IntegerNode:
		return a
	}
	return nil
}

func toFloat(n ast.Node) *ast.FloatNode {
	switch a := n.(type) {
	case *ast.FloatNode:
		return a
	}
	return nil
}

func toBool(n ast.Node) *ast.BoolNode {
	switch a := n.(type) {
	case *ast.BoolNode:
		return a
	}
	return nil
}
