package optimizer

import (
	ast2 "github.com/oarkflow/expr/ast"
	"github.com/oarkflow/expr/conf"
)

func Optimize(node *ast2.Node, config *conf.Config) error {
	ast2.Walk(node, &inArray{})
	for limit := 1000; limit >= 0; limit-- {
		fold := &fold{}
		ast2.Walk(node, fold)
		if fold.err != nil {
			return fold.err
		}
		if !fold.applied {
			break
		}
	}
	if config != nil && len(config.ConstFns) > 0 {
		for limit := 100; limit >= 0; limit-- {
			constExpr := &constExpr{
				fns: config.ConstFns,
			}
			ast2.Walk(node, constExpr)
			if constExpr.err != nil {
				return constExpr.err
			}
			if !constExpr.applied {
				break
			}
		}
	}
	ast2.Walk(node, &inRange{})
	ast2.Walk(node, &constRange{})
	ast2.Walk(node, &filterMap{})
	ast2.Walk(node, &filterLen{})
	ast2.Walk(node, &filterLast{})
	ast2.Walk(node, &filterFirst{})
	return nil
}
