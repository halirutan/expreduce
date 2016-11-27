package cas

import "bytes"
import "math/big"

func (this *Expression) ToStringList() string {
	var buffer bytes.Buffer
	buffer.WriteString("{")
	for i, e := range this.Parts[1:] {
		buffer.WriteString(e.String())
		if i != len(this.Parts[1:])-1 {
			buffer.WriteString(", ")
		}
	}
	buffer.WriteString("}")
	return buffer.String()
}

func (this *Expression) EvalLength(es *EvalState) Ex {
	if len(this.Parts) != 2 {
		return this
	}

	list, isList := HeadAssertion(this.Parts[1], "List")
	if isList {
		return &Integer{big.NewInt(int64(len(list.Parts) - 1))}
	}
	return this
}

func (this *Expression) EvalTable(es *EvalState) Ex {
	if len(this.Parts) >= 3 {
		mis, isOk := MultiIterSpecFromLists(this.Parts[2:])
		if isOk {
			// Simulate evaluation within Block[]
			mis.TakeVarSnapshot(es)
			toReturn := &Expression{[]Ex{&Symbol{"List"}}}
			for mis.Cont() {
				mis.DefineCurrent(es)
				toReturn.Parts = append(toReturn.Parts, this.Parts[1].DeepCopy().Eval(es))
				es.Debugf("%v\n", toReturn)
				mis.Next()
			}
			mis.RestoreVarSnapshot(es)
			return toReturn
		}
	}
	return this
}

type IterSpec struct {
	i       Ex
	iName   string
	iMin    *Integer
	iMax    *Integer
	curr    int64
	iMaxInt int64
}

func IterSpecFromList(listEx Ex) (is IterSpec, isOk bool) {
	list, isList := HeadAssertion(listEx, "List")
	if isList {
		iOk, iMinOk, iMaxOk := false, false, false
		if len(list.Parts) > 2 {
			iAsSymbol, iIsSymbol := list.Parts[1].(*Symbol)
			if iIsSymbol {
				iOk = true
				is.i = iAsSymbol
				is.iName = iAsSymbol.Name
			}
			iAsExpression, iIsExpression := list.Parts[1].(*Expression)
			if iIsExpression {
				headAsSymbol, headIsSymbol := iAsExpression.Parts[0].(*Symbol)
				if headIsSymbol {
					iOk = true
					is.i = iAsExpression
					is.iName = headAsSymbol.Name
				}
			}
		}
		if len(list.Parts) == 3 {
			is.iMin, iMinOk = &Integer{big.NewInt(1)}, true
			is.iMax, iMaxOk = list.Parts[2].(*Integer)
		} else if len(list.Parts) == 4 {
			is.iMin, iMinOk = list.Parts[2].(*Integer)
			is.iMax, iMaxOk = list.Parts[3].(*Integer)
		}
		if iOk && iMinOk && iMaxOk {
			is.Reset()
			return is, true
		}
	}
	return is, false
}

func (this *IterSpec) Reset() {
	this.curr = this.iMin.Val.Int64()
	this.iMaxInt = this.iMax.Val.Int64()
}

func (this *IterSpec) Next() {
	this.curr++
}

func (this *IterSpec) Cont() bool {
	return this.curr <= this.iMaxInt
}

type MultiIterSpec struct {
	iSpecs []IterSpec
	origDefs []Ex
	isOrigDefs []bool
	cont bool
}

func MultiIterSpecFromLists(lists []Ex) (mis MultiIterSpec, isOk bool) {
	// Retrieve variables of iteration
	mis.cont = true
	for i := range(lists) {
		is, isOk := IterSpecFromList(lists[i])
		if !isOk {
			return mis, false
		}
		mis.iSpecs = append(mis.iSpecs, is)
	}
	return mis, true
}

func (this *MultiIterSpec) Next() {
	for i := len(this.iSpecs)-1; i >= 0; i-- {
		this.iSpecs[i].Next()
		if this.iSpecs[i].Cont() {
			return
		}
		this.iSpecs[i].Reset()
	}
	this.cont = false
}

func (this *MultiIterSpec) Cont() bool {
	return this.cont
}

func (this *MultiIterSpec) TakeVarSnapshot(es *EvalState) {
	this.origDefs = make([]Ex, len(this.iSpecs))
	this.isOrigDefs = make([]bool, len(this.iSpecs))
	for i := range(this.iSpecs) {
		this.origDefs[i], this.isOrigDefs[i] = es.GetDef(this.iSpecs[i].iName, this.iSpecs[i].i)
	}
}

func (this *MultiIterSpec) RestoreVarSnapshot(es *EvalState) {
	for i := range(this.iSpecs) {
		if this.isOrigDefs[i] {
			es.Define(this.iSpecs[i].iName, this.iSpecs[i].i, this.origDefs[i])
		} else {
			es.Clear(this.iSpecs[i].iName)
		}
	}
}

func (this *MultiIterSpec) DefineCurrent(es *EvalState) {
	for i := range(this.iSpecs) {
		es.Define(this.iSpecs[i].iName, this.iSpecs[i].i, &Integer{big.NewInt(this.iSpecs[i].curr)})
	}
}

func (this *Expression) EvalIterationFunc(es *EvalState, init Ex, op string) Ex {
	if len(this.Parts) >= 3 {
		mis, isOk := MultiIterSpecFromLists(this.Parts[2:])
		if isOk {
			// Simulate evaluation within Block[]
			mis.TakeVarSnapshot(es)
			var toReturn Ex = init
			for mis.Cont() {
				mis.DefineCurrent(es)
				toReturn = (&Expression{[]Ex{&Symbol{op}, toReturn, this.Parts[1].DeepCopy().Eval(es)}}).Eval(es)
				mis.Next()
			}
			mis.RestoreVarSnapshot(es)
			return toReturn
		}
	}
	return this
}

func (this *Expression) EvalSum(es *EvalState) Ex {
	return this.EvalIterationFunc(es, &Integer{big.NewInt(0)}, "Plus")
}

func (this *Expression) EvalProduct(es *EvalState) Ex {
	return this.EvalIterationFunc(es, &Integer{big.NewInt(1)}, "Times")
}

func MemberQ(components []Ex, item Ex, cl *CASLogger) bool {
	for _, part := range components {
		if matchq, _ := IsMatchQ(part, item, EmptyPD(), cl); matchq {
			return true
		}
	}
	return false
}

func (this *Expression) EvalMemberQ(es *EvalState) Ex {
	if len(this.Parts) != 3 {
		return this
	}
	list, isList := HeadAssertion(this.Parts[1], "List")
	if isList {
		if MemberQ(list.Parts, this.Parts[2], &es.CASLogger) {
			return &Symbol{"True"}
		}
	}
	return &Symbol{"False"}
}

func (this *Expression) EvalMap(es *EvalState) Ex {
	if len(this.Parts) != 3 {
		return this
	}

	list, isList := HeadAssertion(this.Parts[2], "List")
	if isList {
		toReturn := &Expression{[]Ex{&Symbol{"List"}}}
		for i := 1; i < len(list.Parts); i++ {
			toReturn.Parts = append(toReturn.Parts, &Expression{[]Ex{
				this.Parts[1].DeepCopy(),
				list.Parts[i].DeepCopy(),
			}})
		}
		return toReturn
	}
	return this.Parts[2]
}

func (this *Expression) EvalArray(es *EvalState) Ex {
	if len(this.Parts) != 3 {
		return this
	}

	nInt, nOk := this.Parts[2].(*Integer)
	if nOk {
		n := nInt.Val.Int64()
		toReturn := &Expression{[]Ex{&Symbol{"List"}}}
		for i := int64(1); i <= n; i++ {
			toReturn.Parts = append(toReturn.Parts, &Expression{[]Ex{
				this.Parts[1].DeepCopy(),
				&Integer{big.NewInt(i)},
			}})
		}
		return toReturn
	}
	return this.Parts[2]
}

func (this *Expression) EvalCases(es *EvalState) Ex {
	if len(this.Parts) != 3 {
		return this
	}

	list, isList := HeadAssertion(this.Parts[1], "List")
	if isList {
		toReturn := &Expression{[]Ex{&Symbol{"List"}}}

		for i := 1; i < len(list.Parts); i++ {
			if matchq, _ := IsMatchQ(list.Parts[i], this.Parts[2], EmptyPD(), &es.CASLogger); matchq {
				toReturn.Parts = append(toReturn.Parts, list.Parts[i])
			}
		}

		return toReturn
	}
	return this
}
