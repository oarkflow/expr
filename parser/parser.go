package parser

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oarkflow/expr/ast"
	"github.com/oarkflow/expr/builtin"
	"github.com/oarkflow/expr/conf"
	"github.com/oarkflow/expr/file"
	lexer2 "github.com/oarkflow/expr/parser/lexer"
	"github.com/oarkflow/expr/parser/operator"
	"github.com/oarkflow/expr/parser/utils"
)

var predicates = map[string]struct {
	arity int
}{
	"all":           {2},
	"none":          {2},
	"any":           {2},
	"one":           {2},
	"filter":        {2},
	"map":           {2},
	"count":         {2},
	"find":          {2},
	"findIndex":     {2},
	"findLast":      {2},
	"findLastIndex": {2},
	"groupBy":       {2},
	"reduce":        {3},
}

type parser struct {
	tokens  []lexer2.Token
	current lexer2.Token
	pos     int
	err     *file.Error
	depth   int // closure call depth
	config  *conf.Config
}

type Tree struct {
	Node   ast.Node
	Source *file.Source
}

func Parse(input string) (*Tree, error) {
	return ParseWithConfig(input, &conf.Config{
		Disabled: map[string]bool{},
	})
}

func ParseWithConfig(input string, config *conf.Config) (*Tree, error) {
	source := file.NewSource(input)

	tokens, err := lexer2.Lex(source)
	if err != nil {
		return nil, err
	}

	p := &parser{
		tokens:  tokens,
		current: tokens[0],
		config:  config,
	}

	node := p.parseExpression(0)

	if !p.current.Is(lexer2.EOF) {
		p.error("unexpected token %v", p.current)
	}

	if p.err != nil {
		return nil, p.err.Bind(source)
	}

	return &Tree{
		Node:   node,
		Source: source,
	}, nil
}

func (p *parser) error(format string, args ...any) {
	p.errorAt(p.current, format, args...)
}

func (p *parser) errorAt(token lexer2.Token, format string, args ...any) {
	if p.err == nil { // show first error
		p.err = &file.Error{
			Location: token.Location,
			Message:  fmt.Sprintf(format, args...),
		}
	}
}

func (p *parser) next() {
	p.pos++
	if p.pos >= len(p.tokens) {
		p.error("unexpected end of expression")
		return
	}
	p.current = p.tokens[p.pos]
}

func (p *parser) expect(kind lexer2.Kind, values ...string) {
	if p.current.Is(kind, values...) {
		p.next()
		return
	}
	p.error("unexpected token %v", p.current)
}

// parse functions

func (p *parser) parseExpression(precedence int) ast.Node {
	if precedence == 0 {
		if p.current.Is(lexer2.Operator, "let") {
			return p.parseVariableDeclaration()
		}
	}

	nodeLeft := p.parsePrimary()

	prevOperator := ""
	opToken := p.current
	for opToken.Is(lexer2.Operator) && p.err == nil {
		negate := false
		var notToken lexer2.Token

		// Handle "not *" operator, like "not in" or "not contains".
		if opToken.Is(lexer2.Operator, "not") {
			p.next()
			notToken = p.current
			negate = true
			opToken = p.current
		}

		if op, ok := operator.Binary[opToken.Value]; ok {
			if op.Precedence >= precedence {
				p.next()

				if opToken.Value == "|" {
					nodeLeft = p.parsePipe(nodeLeft)
					goto next
				}

				if prevOperator == "??" && opToken.Value != "??" && !opToken.Is(lexer2.Bracket, "(") {
					p.errorAt(opToken, "Operator (%v) and coalesce expressions (??) cannot be mixed. Wrap either by parentheses.", opToken.Value)
					break
				}

				var nodeRight ast.Node
				if op.Associativity == operator.Left {
					nodeRight = p.parseExpression(op.Precedence + 1)
				} else {
					nodeRight = p.parseExpression(op.Precedence)
				}

				nodeLeft = &ast.BinaryNode{
					Operator: opToken.Value,
					Left:     nodeLeft,
					Right:    nodeRight,
				}
				nodeLeft.SetLocation(opToken.Location)

				if negate {
					nodeLeft = &ast.UnaryNode{
						Operator: "not",
						Node:     nodeLeft,
					}
					nodeLeft.SetLocation(notToken.Location)
				}

				goto next
			}
		}
		break

	next:
		prevOperator = opToken.Value
		opToken = p.current
	}

	if precedence == 0 {
		nodeLeft = p.parseConditional(nodeLeft)
	}

	return nodeLeft
}

func (p *parser) parseVariableDeclaration() ast.Node {
	p.expect(lexer2.Operator, "let")
	variableName := p.current
	p.expect(lexer2.Identifier)
	p.expect(lexer2.Operator, "=")
	value := p.parseExpression(0)
	p.expect(lexer2.Operator, ";")
	node := p.parseExpression(0)
	let := &ast.VariableDeclaratorNode{
		Name:  variableName.Value,
		Value: value,
		Expr:  node,
	}
	let.SetLocation(variableName.Location)
	return let
}

func (p *parser) parseConditional(node ast.Node) ast.Node {
	var expr1, expr2 ast.Node
	for p.current.Is(lexer2.Operator, "?") && p.err == nil {
		p.next()

		if !p.current.Is(lexer2.Operator, ":") {
			expr1 = p.parseExpression(0)
			p.expect(lexer2.Operator, ":")
			expr2 = p.parseExpression(0)
		} else {
			p.next()
			expr1 = node
			expr2 = p.parseExpression(0)
		}

		node = &ast.ConditionalNode{
			Cond: node,
			Exp1: expr1,
			Exp2: expr2,
		}
	}
	return node
}

func (p *parser) parsePrimary() ast.Node {
	token := p.current

	if token.Is(lexer2.Operator) {
		if op, ok := operator.Unary[token.Value]; ok {
			p.next()
			expr := p.parseExpression(op.Precedence)
			node := &ast.UnaryNode{
				Operator: token.Value,
				Node:     expr,
			}
			node.SetLocation(token.Location)
			return p.parsePostfixExpression(node)
		}
	}

	if token.Is(lexer2.Bracket, "(") {
		p.next()
		expr := p.parseExpression(0)
		p.expect(lexer2.Bracket, ")") // "an opened parenthesis is not properly closed"
		return p.parsePostfixExpression(expr)
	}

	if p.depth > 0 {
		if token.Is(lexer2.Operator, "#") || token.Is(lexer2.Operator, ".") {
			name := ""
			if token.Is(lexer2.Operator, "#") {
				p.next()
				if p.current.Is(lexer2.Identifier) {
					name = p.current.Value
					p.next()
				}
			}
			node := &ast.PointerNode{Name: name}
			node.SetLocation(token.Location)
			return p.parsePostfixExpression(node)
		}
	} else {
		if token.Is(lexer2.Operator, "#") || token.Is(lexer2.Operator, ".") {
			p.error("cannot use pointer accessor outside closure")
		}
	}

	return p.parseSecondary()
}

func (p *parser) parseSecondary() ast.Node {
	var node ast.Node
	token := p.current

	switch token.Kind {

	case lexer2.Identifier:
		p.next()
		switch token.Value {
		case "true":
			node := &ast.BoolNode{Value: true}
			node.SetLocation(token.Location)
			return node
		case "false":
			node := &ast.BoolNode{Value: false}
			node.SetLocation(token.Location)
			return node
		case "nil":
			node := &ast.NilNode{}
			node.SetLocation(token.Location)
			return node
		default:
			node = p.parseCall(token)
		}

	case lexer2.Number:
		p.next()
		value := strings.Replace(token.Value, "_", "", -1)
		if strings.Contains(value, "x") {
			number, err := strconv.ParseInt(value, 0, 64)
			if err != nil {
				p.error("invalid hex literal: %v", err)
			}
			if number > math.MaxInt {
				p.error("integer literal is too large")
				return nil
			}
			node := &ast.IntegerNode{Value: int(number)}
			node.SetLocation(token.Location)
			return node
		} else if strings.ContainsAny(value, ".eE") {
			number, err := strconv.ParseFloat(value, 64)
			if err != nil {
				p.error("invalid float literal: %v", err)
			}
			node := &ast.FloatNode{Value: number}
			node.SetLocation(token.Location)
			return node
		} else {
			number, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				p.error("invalid integer literal: %v", err)
			}
			if number > math.MaxInt {
				p.error("integer literal is too large")
				return nil
			}
			node := &ast.IntegerNode{Value: int(number)}
			node.SetLocation(token.Location)
			return node
		}

	case lexer2.String:
		p.next()
		node := &ast.StringNode{Value: token.Value}
		node.SetLocation(token.Location)
		return node

	default:
		if token.Is(lexer2.Bracket, "[") {
			node = p.parseArrayExpression(token)
		} else if token.Is(lexer2.Bracket, "{") {
			node = p.parseMapExpression(token)
		} else {
			p.error("unexpected token %v", token)
		}
	}

	return p.parsePostfixExpression(node)
}

func (p *parser) parseCall(token lexer2.Token) ast.Node {
	var node ast.Node
	if p.current.Is(lexer2.Bracket, "(") {
		var arguments []ast.Node

		if b, ok := predicates[token.Value]; ok {
			p.expect(lexer2.Bracket, "(")

			// TODO: Refactor parser to use builtin.Builtins instead of predicates map.

			if b.arity == 1 {
				arguments = make([]ast.Node, 1)
				arguments[0] = p.parseExpression(0)
			} else if b.arity == 2 {
				arguments = make([]ast.Node, 2)
				arguments[0] = p.parseExpression(0)
				p.expect(lexer2.Operator, ",")
				arguments[1] = p.parseClosure()
			}

			if token.Value == "reduce" {
				arguments = make([]ast.Node, 2)
				arguments[0] = p.parseExpression(0)
				p.expect(lexer2.Operator, ",")
				arguments[1] = p.parseClosure()
				if p.current.Is(lexer2.Operator, ",") {
					p.next()
					arguments = append(arguments, p.parseExpression(0))
				}
			}

			p.expect(lexer2.Bracket, ")")

			node = &ast.BuiltinNode{
				Name:      token.Value,
				Arguments: arguments,
			}
			node.SetLocation(token.Location)
		} else if _, ok := builtin.Index[token.Value]; ok && !p.config.Disabled[token.Value] {
			node = &ast.BuiltinNode{
				Name:      token.Value,
				Arguments: p.parseArguments(),
			}
			node.SetLocation(token.Location)
		} else {
			callee := &ast.IdentifierNode{Value: token.Value}
			callee.SetLocation(token.Location)
			node = &ast.CallNode{
				Callee:    callee,
				Arguments: p.parseArguments(),
			}
			node.SetLocation(token.Location)
		}
	} else {
		node = &ast.IdentifierNode{Value: token.Value}
		node.SetLocation(token.Location)
	}
	return node
}

func (p *parser) parseClosure() ast.Node {
	startToken := p.current
	expectClosingBracket := false
	if p.current.Is(lexer2.Bracket, "{") {
		p.next()
		expectClosingBracket = true
	}

	p.depth++
	node := p.parseExpression(0)
	p.depth--

	if expectClosingBracket {
		p.expect(lexer2.Bracket, "}")
	}
	closure := &ast.ClosureNode{
		Node: node,
	}
	closure.SetLocation(startToken.Location)
	return closure
}

func (p *parser) parseArrayExpression(token lexer2.Token) ast.Node {
	nodes := make([]ast.Node, 0)

	p.expect(lexer2.Bracket, "[")
	for !p.current.Is(lexer2.Bracket, "]") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(lexer2.Operator, ",")
			if p.current.Is(lexer2.Bracket, "]") {
				goto end
			}
		}
		node := p.parseExpression(0)
		nodes = append(nodes, node)
	}
end:
	p.expect(lexer2.Bracket, "]")

	node := &ast.ArrayNode{Nodes: nodes}
	node.SetLocation(token.Location)
	return node
}

func (p *parser) parseMapExpression(token lexer2.Token) ast.Node {
	p.expect(lexer2.Bracket, "{")

	nodes := make([]ast.Node, 0)
	for !p.current.Is(lexer2.Bracket, "}") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(lexer2.Operator, ",")
			if p.current.Is(lexer2.Bracket, "}") {
				goto end
			}
			if p.current.Is(lexer2.Operator, ",") {
				p.error("unexpected token %v", p.current)
			}
		}

		var key ast.Node
		// Map key can be one of:
		//  * number
		//  * string
		//  * identifier, which is equivalent to a string
		//  * expression, which must be enclosed in parentheses -- (1 + 2)
		if p.current.Is(lexer2.Number) || p.current.Is(lexer2.String) || p.current.Is(lexer2.Identifier) {
			key = &ast.StringNode{Value: p.current.Value}
			key.SetLocation(token.Location)
			p.next()
		} else if p.current.Is(lexer2.Bracket, "(") {
			key = p.parseExpression(0)
		} else {
			p.error("a map key must be a quoted string, a number, a identifier, or an expression enclosed in parentheses (unexpected token %v)", p.current)
		}

		p.expect(lexer2.Operator, ":")

		node := p.parseExpression(0)
		pair := &ast.PairNode{Key: key, Value: node}
		pair.SetLocation(token.Location)
		nodes = append(nodes, pair)
	}

end:
	p.expect(lexer2.Bracket, "}")

	node := &ast.MapNode{Pairs: nodes}
	node.SetLocation(token.Location)
	return node
}

func (p *parser) parsePostfixExpression(node ast.Node) ast.Node {
	postfixToken := p.current
	for (postfixToken.Is(lexer2.Operator) || postfixToken.Is(lexer2.Bracket)) && p.err == nil {
		if postfixToken.Value == "." || postfixToken.Value == "?." {
			p.next()

			propertyToken := p.current
			p.next()

			if propertyToken.Kind != lexer2.Identifier &&
				// Operators like "not" and "matches" are valid methods or property names.
				(propertyToken.Kind != lexer2.Operator || !utils.IsValidIdentifier(propertyToken.Value)) {
				p.error("expected name")
			}

			property := &ast.StringNode{Value: propertyToken.Value}
			property.SetLocation(propertyToken.Location)

			chainNode, isChain := node.(*ast.ChainNode)
			optional := postfixToken.Value == "?."

			if isChain {
				node = chainNode.Node
			}

			memberNode := &ast.MemberNode{
				Node:     node,
				Property: property,
				Optional: optional,
			}
			memberNode.SetLocation(propertyToken.Location)

			if p.current.Is(lexer2.Bracket, "(") {
				node = &ast.CallNode{
					Callee:    memberNode,
					Arguments: p.parseArguments(),
				}
				node.SetLocation(propertyToken.Location)
			} else {
				node = memberNode
			}

			if isChain || optional {
				node = &ast.ChainNode{Node: node}
			}

		} else if postfixToken.Value == "[" {
			p.next()
			var from, to ast.Node

			if p.current.Is(lexer2.Operator, ":") { // slice without from [:1]
				p.next()

				if !p.current.Is(lexer2.Bracket, "]") { // slice without from and to [:]
					to = p.parseExpression(0)
				}

				node = &ast.SliceNode{
					Node: node,
					To:   to,
				}
				node.SetLocation(postfixToken.Location)
				p.expect(lexer2.Bracket, "]")

			} else {

				from = p.parseExpression(0)

				if p.current.Is(lexer2.Operator, ":") {
					p.next()

					if !p.current.Is(lexer2.Bracket, "]") { // slice without to [1:]
						to = p.parseExpression(0)
					}

					node = &ast.SliceNode{
						Node: node,
						From: from,
						To:   to,
					}
					node.SetLocation(postfixToken.Location)
					p.expect(lexer2.Bracket, "]")

				} else {
					// Slice operator [:] was not found,
					// it should be just an index node.
					node = &ast.MemberNode{
						Node:     node,
						Property: from,
					}
					node.SetLocation(postfixToken.Location)
					p.expect(lexer2.Bracket, "]")
				}
			}
		} else {
			break
		}
		postfixToken = p.current
	}
	return node
}

func (p *parser) parsePipe(node ast.Node) ast.Node {
	identifier := p.current
	p.expect(lexer2.Identifier)

	arguments := []ast.Node{node}

	if b, ok := predicates[identifier.Value]; ok {
		p.expect(lexer2.Bracket, "(")

		// TODO: Refactor parser to use builtin.Builtins instead of predicates map.

		if b.arity == 2 {
			arguments = append(arguments, p.parseClosure())
		}

		if identifier.Value == "reduce" {
			arguments = append(arguments, p.parseClosure())
			if p.current.Is(lexer2.Operator, ",") {
				p.next()
				arguments = append(arguments, p.parseExpression(0))
			}
		}

		p.expect(lexer2.Bracket, ")")

		node = &ast.BuiltinNode{
			Name:      identifier.Value,
			Arguments: arguments,
		}
		node.SetLocation(identifier.Location)
	} else if _, ok := builtin.Index[identifier.Value]; ok {
		arguments = append(arguments, p.parseArguments()...)

		node = &ast.BuiltinNode{
			Name:      identifier.Value,
			Arguments: arguments,
		}
		node.SetLocation(identifier.Location)
	} else {
		callee := &ast.IdentifierNode{Value: identifier.Value}
		callee.SetLocation(identifier.Location)

		arguments = append(arguments, p.parseArguments()...)

		node = &ast.CallNode{
			Callee:    callee,
			Arguments: arguments,
		}
		node.SetLocation(identifier.Location)
	}

	return node
}

func (p *parser) parseArguments() []ast.Node {
	p.expect(lexer2.Bracket, "(")
	nodes := make([]ast.Node, 0)
	for !p.current.Is(lexer2.Bracket, ")") && p.err == nil {
		if len(nodes) > 0 {
			p.expect(lexer2.Operator, ",")
		}
		node := p.parseExpression(0)
		nodes = append(nodes, node)
	}
	p.expect(lexer2.Bracket, ")")

	return nodes
}
