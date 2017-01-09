package expreduce

func IsMatchQRational(a *Rational, b *Expression, pm *PDManager, cl *CASLogger) (bool, *PDManager) {
	return IsMatchQ(
		&Expression{[]Ex{
			&Symbol{"Rational"},
			&Integer{a.Num},
			&Integer{a.Den},
		}},
		b, pm, cl)
}

func IsMatchQ(a Ex, b Ex, pm *PDManager, cl *CASLogger) (bool, *PDManager) {
	// Special case for Except
	except, isExcept := HeadAssertion(b, "Except")
	if isExcept {
		if len(except.Parts) == 2 {
			matchq, _ := IsMatchQ(a, except.Parts[1], EmptyPD(), cl)
			return !matchq, pm
		} else if len(except.Parts) == 3 {
			matchq, _ := IsMatchQ(a, except.Parts[1], EmptyPD(), cl)
			if !matchq {
				matchqb, newPm := IsMatchQ(a, except.Parts[2], pm, cl)
				return matchqb, newPm
			}
			return false, pm
		}
	}
	// Special case for Alternatives
	alts, isAlts := HeadAssertion(b, "Alternatives")
	if isAlts {
		for _, alt := range alts.Parts[1:] {
			matchq, newPD := IsMatchQ(a, alt, EmptyPD(), cl)
			if matchq {
				return matchq, newPD
			}
		}
		return false, pm
	}
	// Special case for PatternTest
	patternTest, isPT := HeadAssertion(b, "PatternTest")
	if isPT {
		if len(patternTest.Parts) == 3 {
			matchq, newPD := IsMatchQ(a, patternTest.Parts[1], EmptyPD(), cl)
			if matchq {
				tmpEs := NewEvalStateNoLog(true)
				res := (&Expression{[]Ex{
					patternTest.Parts[2],
					a,
				}}).Eval(tmpEs)
				resSymbol, resIsSymbol := res.(*Symbol)
				if resIsSymbol {
					if resSymbol.Name == "True" {
						return true, newPD
					}
				}
			}
			return false, pm
		}
	}
	// Special case for Condition
	condition, isCond := HeadAssertion(b, "Condition")
	if isCond {
		if len(condition.Parts) == 3 {
			matchq, newPD := IsMatchQ(a, condition.Parts[1], EmptyPD(), cl)
			if matchq {
				tmpEs := NewEvalStateNoLog(true)
				res := condition.Parts[2].DeepCopy()
				res = ReplacePD(res, cl, newPD).Eval(tmpEs)
				resSymbol, resIsSymbol := res.(*Symbol)
				if resIsSymbol {
					if resSymbol.Name == "True" {
						return true, newPD
					}
				}
			}
		}
	}

	// Continue normally
	pm = CopyPD(pm)
	_, aIsFlt := a.(*Flt)
	_, aIsInteger := a.(*Integer)
	_, aIsString := a.(*String)
	_, aIsSymbol := a.(*Symbol)
	aRational, aIsRational := a.(*Rational)
	bRational, bIsRational := b.(*Rational)
	aExpression, aIsExpression := a.(*Expression)
	bExpression, bIsExpression := b.(*Expression)

	// This initial value is just a randomly chosen placeholder
	// TODO, convert headStr to symbol type, have Ex implement getHead() Symbol
	headStr := "Unknown"
	if aIsFlt {
		headStr = "Real"
	} else if aIsInteger {
		headStr = "Integer"
	} else if aIsString {
		headStr = "String"
	} else if aIsExpression {
		headStr = aExpression.Parts[0].String()
	} else if aIsSymbol {
		headStr = "Symbol"
	} else if aIsRational {
		headStr = "Rational"
	}

	if IsBlankTypeOnly(b) {
		ibtc, ibtcNewPDs := IsBlankTypeCapturing(b, a, headStr, pm, cl)
		if ibtc {
			return true, ibtcNewPDs
		}
		return false, EmptyPD()
	}

	// Handle special case for matching Rational[a_Integer, b_Integer]
	if aIsRational && bIsExpression {
		return IsMatchQRational(aRational, bExpression, pm, cl)
	} else if aIsExpression && bIsRational {
		return IsMatchQRational(bRational, aExpression, pm, cl)
	}

	if aIsFlt || aIsInteger || aIsString || aIsSymbol || aIsRational {
		return IsSameQ(a, b, cl), EmptyPD()
	} else if !(aIsExpression && bIsExpression) {
		return false, EmptyPD()
	}

	aExpressionSym, aExpressionSymOk := aExpression.Parts[0].(*Symbol)
	bExpressionSym, bExpressionSymOk := bExpression.Parts[0].(*Symbol)
	if aExpressionSymOk && bExpressionSymOk {
		if aExpressionSym.Name == bExpressionSym.Name {
			if IsOrderless(aExpressionSym) {
				return OrderlessIsMatchQ(aExpression.Parts[1:len(aExpression.Parts)], bExpression.Parts[1:len(bExpression.Parts)], pm, cl)
			}
		}
	}

	return NonOrderlessIsMatchQ(aExpression.Parts, bExpression.Parts, pm, cl)
}

func IsSameQ(a Ex, b Ex, cl *CASLogger) bool {
	_, aIsFlt := a.(*Flt)
	_, bIsFlt := b.(*Flt)
	_, aIsInteger := a.(*Integer)
	_, bIsInteger := b.(*Integer)
	_, aIsString := a.(*String)
	_, bIsString := b.(*String)
	_, aIsSymbol := a.(*Symbol)
	_, bIsSymbol := b.(*Symbol)
	_, aIsRational := a.(*Rational)
	_, bIsRational := b.(*Rational)
	aExpression, aIsExpression := a.(*Expression)
	bExpression, bIsExpression := b.(*Expression)

	if (aIsFlt && bIsFlt) || (aIsString && bIsString) || (aIsInteger && bIsInteger) || (aIsSymbol && bIsSymbol) || (aIsRational && bIsRational) {
		// a and b are identical raw types
		return a.IsEqual(b, cl) == "EQUAL_TRUE"
	} else if aIsExpression && bIsExpression {
		// a and b are both expressions
		return FunctionIsSameQ(aExpression.Parts, bExpression.Parts, cl)
	}

	// This should never happen
	return false
}

func GetComparisonDefinitions() (defs []Definition) {
	defs = append(defs, Definition{
		Name: "Equal",
		Usage: "`lhs == rhs` evaluates to True or False if equality or inequality is known.",
		toString: func(this *Expression, form string) (bool, string) {
			return ToStringInfixAdvanced(this.Parts[1:], " == ", true, "", "", form)
		},
		legacyEvalFn: func(this *Expression, es *EvalState) Ex {
			if len(this.Parts) != 3 {
				return this
			}

			var isequal string = this.Parts[1].IsEqual(this.Parts[2], &es.CASLogger)
			if isequal == "EQUAL_UNK" {
				return this
			} else if isequal == "EQUAL_TRUE" {
				return &Symbol{"True"}
			} else if isequal == "EQUAL_FALSE" {
				return &Symbol{"False"}
			}

			return &Expression{[]Ex{&Symbol{"Error"}, &String{"Unexpected equality return value."}}}
		},
		SimpleExamples: []TestInstruction{
			&TestComment{"Expressions known to be equal will evaluate to True:"},
			&StringTest{"True", "9*x==x*9"},
			&TestComment{"Sometimes expressions may or may not be equal, or Expreduce does not know how to test for equality. In these cases, the statement will remain unevaluated:"},
			&StringTest{"((9 * x)) == ((10 * x))", "9*x==x*10"},

			&TestComment{"Equal considers Integers and Reals that are close enough to be equal:"},
			&StringTest{"5", "tmp=5"},
			&StringTest{"True", "tmp==5"},
			&StringTest{"True", "tmp==5."},
			&StringTest{"True", "tmp==5.00000"},

			&TestComment{"Equal can test for Rational equality:"},
			&StringTest{"False", "4/3==3/2"},
			&StringTest{"True", "4/3==8/6"},
		},
		FurtherExamples: []TestInstruction{
			&StringTest{"True", "If[xx == 2, yy, zz] == If[xx == 2, yy, zz]"},
			&TestComment{"Equal does not match patterns:"},
			&SameTest{"{1, 2, 3} == _List", "{1, 2, 3} == _List"},
			&TestComment{"This functionality is reserved for MatchQ:"},
			&SameTest{"True", "MatchQ[{1, 2, 3}, _List]"},
		},
		Tests: []TestInstruction{

			&StringTest{"5", "tmp=5"},
			&StringTest{"True", "tmp==5"},
			&StringTest{"True", "5==tmp"},
			&StringTest{"False", "tmp==6"},
			&StringTest{"False", "6==tmp"},

			&StringTest{"(a) == (b)", "a==b"},
			&StringTest{"True", "a==a"},
			&StringTest{"(a) == (2)", "a==2"},
			&StringTest{"(2) == (a)", "2==a"},
			&StringTest{"(2) == ((a + b))", "2==a+b"},
			&StringTest{"(2.) == (a)", "2.==a"},
			&StringTest{"(2^k) == (a)", "2^k==a"},
			&StringTest{"(2^k) == (2^a)", "2^k==2^a"},
			&StringTest{"(2^k) == ((2 + k))", "2^k==k+2"},
			&StringTest{"(k) == ((2 * k))", "k==2*k"},
			&StringTest{"((2 * k)) == (k)", "2*k==k"},
			&StringTest{"True", "1+1==2"},
			&StringTest{"(y) == ((b + (m * x)))", "y==m*x+b"},

			&StringTest{"True", "1==1."},
			&StringTest{"True", "1.==1"},

			&StringTest{"True", "(x==2)==(x==2)"},
			&StringTest{"True", "(x==2.)==(x==2)"},
			&StringTest{"True", "(x===2.)==(x===2)"},

			&StringTest{"(If[(xx) == (3), yy, zz]) == (If[(xx) == (2), yy, zz])", "If[xx == 3, yy, zz] == If[xx == 2, yy, zz]"},

			&StringTest{"True", "(1 == 2) == (2 == 3)"},
			&StringTest{"False", "(1 == 2) == (2 == 2)"},

			&SameTest{"True", "foo[x == 2, y, x] == foo[x == 2, y, x]"},
			&SameTest{"True", "foo[x == 2, y, x] == foo[x == 2., y, x]"},
			&SameTest{"foo[x == 2, y, x] == foo[x == 2., y, y]", "foo[x == 2, y, x] == foo[x == 2., y, y]"},
			&SameTest{"foo[x == 2, y, x] == bar[x == 2, y, x]", "foo[x == 2, y, x] == bar[x == 2, y, x]"},

			&StringTest{"(foo[x, y, z]) == (foo[x, y])", "foo[x, y, z] == foo[x, y]"},
			&StringTest{"(foo[x, y, z]) == (foo[x, y, 1])", "foo[x, y, z] == foo[x, y, 1]"},
			&SameTest{"True", "foo[x, y, 1] == foo[x, y, 1]"},
			&SameTest{"True", "foo[x, y, 1.] == foo[x, y, 1]"},
		},
	})
	defs = append(defs, Definition{
		Name: "Unequal",
		Usage: "`lhs != rhs` evaluates to True if inequality is known or False if equality is known.",
		toString: func(this *Expression, form string) (bool, string) {
			return ToStringInfixAdvanced(this.Parts[1:], " != ", true, "", "", form)
		},
		legacyEvalFn: func(this *Expression, es *EvalState) Ex {
			if len(this.Parts) != 3 {
				return this
			}

			var isequal string = this.Parts[1].IsEqual(this.Parts[2], &es.CASLogger)
			if isequal == "EQUAL_UNK" {
				return this
			} else if isequal == "EQUAL_TRUE" {
				return &Symbol{"False"}
			} else if isequal == "EQUAL_FALSE" {
				return &Symbol{"True"}
			}

			return &Expression{[]Ex{&Symbol{"Error"}, &String{"Unexpected equality return value."}}}
		},
		SimpleExamples: []TestInstruction{
			&TestComment{"Expressions known to be unequal will evaluate to True:"},
			&StringTest{"True", "9 != 8"},
			&TestComment{"Sometimes expressions may or may not be unequal, or Expreduce does not know how to test for inequality. In these cases, the statement will remain unevaluated:"},
			&StringTest{"((9 * x)) != ((10 * x))", "9*x != x*10"},

			&TestComment{"Unequal considers Integers and Reals that are close enough to be equal:"},
			&StringTest{"5", "tmp=5"},
			&StringTest{"False", "tmp != 5"},
			&StringTest{"False", "tmp != 5."},
			&StringTest{"False", "tmp != 5.00000"},

			&TestComment{"Unequal can test for Rational inequality:"},
			&StringTest{"True", "4/3 != 3/2"},
			&StringTest{"False", "4/3 != 8/6"},
		},
	})
	defs = append(defs, Definition{
		Name: "SameQ",
		Usage: "`lhs === rhs` evaluates to True if `lhs` and `rhs` are identical after evaluation, False otherwise.",
		toString: func(this *Expression, form string) (bool, string) {
			return ToStringInfixAdvanced(this.Parts[1:], " === ", true, "", "", form)
		},
		legacyEvalFn: func(this *Expression, es *EvalState) Ex {
			if len(this.Parts) != 3 {
				return this
			}

			var issame bool = IsSameQ(this.Parts[1], this.Parts[2], &es.CASLogger)
			if issame {
				return &Symbol{"True"}
			} else {
				return &Symbol{"False"}
			}
		},
		SimpleExamples: []TestInstruction{
			&StringTest{"True", "a===a"},
			&StringTest{"True", "5 === 5"},
			&TestComment{"Unlike Equal, SameQ does not forgive differences between Integers and Reals:"},
			&StringTest{"False", "5 === 5."},
			&TestComment{"SameQ considers the arguments of all expressions and subexpressions:"},
			&SameTest{"True", "foo[x == 2, y, x] === foo[x == 2, y, x]"},
			&SameTest{"False", "foo[x == 2, y, x] === foo[x == 2., y, x]"},
		},
		FurtherExamples: []TestInstruction{
			&TestComment{"SameQ does not match patterns:"},
			&SameTest{"False", "{1, 2, 3} === _List"},
			&TestComment{"This functionality is reserved for MatchQ:"},
			&SameTest{"True", "MatchQ[{1, 2, 3}, _List]"},
		},
		Tests: []TestInstruction{
			&StringTest{"5", "tmp=5"},
			&StringTest{"False", "a===b"},
			&StringTest{"True", "tmp===5"},
			&StringTest{"False", "tmp===5."},
			&StringTest{"True", "1+1===2"},
			&StringTest{"False", "y===m*x+b"},

			&StringTest{"False", "1===1."},
			&StringTest{"False", "1.===1"},

			&StringTest{"True", "(x===2.)===(x===2)"},
			&StringTest{"False", "(x==2.)===(x==2)"},

			&StringTest{"True", "If[xx == 2, yy, zz] === If[xx == 2, yy, zz]"},
			&StringTest{"False", "If[xx == 2, yy, zz] === If[xx == 2., yy, zz]"},
			&StringTest{"False", "If[xx == 3, yy, zz] === If[xx == 2, yy, zz]"},
			&StringTest{"False", "(x == y) === (y == x)"},
			&StringTest{"True", "(x == y) === (x == y)"},

			&SameTest{"False", "foo[x == 2, y, x] === foo[x == 2., y, y]"},
			&SameTest{"False", "foo[x == 2, y, x] === bar[x == 2, y, x]"},

			&SameTest{"False", "foo[x, y, z] === foo[x, y]"},
			&SameTest{"False", "foo[x, y, z] === foo[x, y, 1]"},
			&SameTest{"True", "foo[x, y, 1] === foo[x, y, 1]"},
			&SameTest{"False", "foo[x, y, 1.] === foo[x, y, 1]"},
		},
	})
	defs = append(defs, Definition{
		Name: "MatchQ",
		Usage: "`MatchQ[expr, form]` returns True if `expr` matches `form`, False otherwise.",
		legacyEvalFn: func(this *Expression, es *EvalState) Ex {
			if len(this.Parts) != 3 {
				return this
			}

			if res, _ := IsMatchQ(this.Parts[1], this.Parts[2], EmptyPD(), &es.CASLogger); res {
				return &Symbol{"True"}
			} else {
				return &Symbol{"False"}
			}
		},
		SimpleExamples: []TestInstruction{
			&TestComment{"A `Blank[]` expression matches everything:"},
			&SameTest{"True", "MatchQ[2*x, _]"},
			&TestComment{"Although a more specific pattern would have matched as well:"},
			&SameTest{"True", "MatchQ[2*x, c1_Integer*a_Symbol]"},
			&TestComment{"Since `Times` is `Orderless`, this would work as well:"},
			&SameTest{"True", "MatchQ[x*2, c1_Integer*a_Symbol]"},
			&TestComment{"As would the `FullForm`:"},
			&SameTest{"True", "MatchQ[Times[x, 2], c1_Integer*a_Symbol]"},

			&TestComment{"Named patterns must match the same expression, or the match will fail:"},
			&SameTest{"False", "MatchQ[a + b, x_Symbol + x_Symbol]"},
		},
		FurtherExamples: []TestInstruction{
			&SameTest{"True", "MatchQ[{2^a, a}, {2^x_Symbol, x_Symbol}]"},
			&SameTest{"False", "MatchQ[{2^a, b}, {2^x_Symbol, x_Symbol}]"},
			&TestComment{"`Blank` sequences allow for the matching of multiple objects. `BlankSequence` (__) matches one or more parts of the expression:"},
			&SameTest{"True", "MatchQ[{a, b}, {a, __}]"},
			&SameTest{"False", "MatchQ[{a}, {a, __}]"},
			&TestComment{"`BlankNullSequence` (___) allows for zero or more matches:"},
			&SameTest{"True", "MatchQ[{a}, {a, ___}]"},
		},
		Tests: []TestInstruction{
			&SameTest{"True", "MatchQ[2^x, base_Integer^pow_Symbol]"},
			&SameTest{"True", "MatchQ[2+x, c1_Integer+a_Symbol]"},
			&SameTest{"True", "MatchQ[a + b, x_Symbol + y_Symbol]"},
			&SameTest{"True", "MatchQ[{a,b}, {x_Symbol,y_Symbol}]"},
			&SameTest{"False", "MatchQ[{a,b}, {x_Symbol,x_Symbol}]"},
			// Test speed of OrderlessIsMatchQ
			&SameTest{"Null", "Plus[testa, testb, rest___] := bar + rest"},
			&SameTest{"bar + 1 + a + b + c + d + e + f + g", "Plus[testa,1,testb,a,b,c,d,e,f,g]"},

			&SameTest{"True", "MatchQ[foo[2*x, x], foo[matcha_Integer*matchx_, matchx_]]"},
			&SameTest{"False", "MatchQ[foo[2*x, x], bar[matcha_Integer*matchx_, matchx_]]"},
			&SameTest{"False", "MatchQ[foo[2*x, y], foo[matcha_Integer*matchx_, matchx_]]"},
			&SameTest{"False", "MatchQ[foo[x, 2*y], foo[matcha_Integer*matchx_, matchx_]]"},
		},
	})
	return
}
