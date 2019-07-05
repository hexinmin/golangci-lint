package golinters 

import (
	"context"
	"fmt"
	"go/ast"
	"container/list"
	"log"
	"os"
	"github.com/golangci/golangci-lint/pkg/lint/linter"
	"go/token"
	"github.com/golangci/golangci-lint/pkg/result"
	//"github.com/hexinmin/SqlInjectInspectInGo"
)

type functionPara struct{
	pName string
	pType string
	conflation [] *functionPara
}

func (fp *functionPara) String() string {
	s := fp.pName
	for _, p := range fp.conflation{
		s = s + "#" + (*p).String()
	}
	if s == ""{
		s = "∅"
	}
	return s
}

func (fp *functionPara) conflate(v *functionPara){
	fp.conflation = append(fp.conflation, v)
}


type StateMentAnalysis int 
const (
	_ StateMentAnalysis = iota
	StateMentAnalysis_START
	StateMentAnalysis_FUNCTION
	StateMentAnalysis_FUNCTION_BODY
)

type DbInput struct{
	format * string
	paras [] *functionPara
	next * DbInput
}

func (di *DbInput) toString() string {
	s := ""
	if di.format == nil{
		s = "(nil)"
	}else
	{
		s = fmt.Sprintf("%d:", len(*di.format))
		s += *di.format
	}

	s = s + " " + "["
	for _, p := range di.paras{
		s = s + " " + (*p).String()
	}
	s = s + "]"
    return s
}

func (di *DbInput) String() string {
	dump := di
	s := (*dump).toString()	
	for{
		if (*dump).next == di || (*dump).next== nil{
			break
		}
		dump = (*dump).next
		s = s + " --> " + (*dump).toString()
	}
	return s
}

func (di *DbInput) updateForAdd(){
	if di.Empty(){
		panic(1)
	}
	if len(di.paras) > 0 && ((*di).format == nil){
		s := "%s"
		di.format = &s
	}
}

func (di *DbInput) Empty() bool {
	return di.format == nil && len(di.paras) == 0 && di.next == nil
}

func (di *DbInput) appendTail(v *DbInput) *DbInput {
	// get tail
	if (*di).next == nil{
		panic(1)
	}
	tail := (*di).next
	for{
		if (*tail).next == di{
			break
		}
		tail = (*tail).next
	}
	(*tail).next = v
	(*v).next = di
	return di
}

func (di *DbInput) likeStringJoin(v *DbInput) *DbInput {
	// deep copy
	if (*di).next == nil{
		panic(1)
	}
	n1 := (*di).next
	n2 := (*n1).next
	for{
		if n1 == n2 || n2 == di{
			break
		}
		newV := &DbInput{
			format: (*v).format,
			paras : (*v).paras,
		}
		(*n1).next = newV
		(*newV).next = n2
		n1 = n2
		n2 = (*n1).next
	}
	return di
}

func (di *DbInput) filterEmptyPara(){
	c := di.getParaCountFromFormat()
	for i:= len(di.paras); i < c; i++{
		di.paras = append(di.paras, nil)
	}
}

func (di *DbInput) getParaCountFromFormat() int {
	count := 0
	var pre rune
	for i, c := range *di.format{
		if i > 0 && c == '%' && pre != '\\' {
			count = count + 1
		}
		pre = c
	}
	return count
}

func (di *DbInput) getFirstEmpty(start int) (int, bool){
	for i , p := range di.paras{
		if i >= start && p == nil{
			return i, true
		}
	}
	return 0, false
}

// getFormatPos index from 0, only for %x
func (di *DbInput) getFormatPos(index int) (int, rune, bool){
	count := 0
	var pre rune
	found := false
	for i, c := range *di.format{
		if found{
			return i - 1, c, true
		}
		if (i > 0 && c == '%' && pre != '\\') ||
			(i == 0 && c == '%') {
			if count == index{
				//return i, pre, true
				found = true
			}
			count = count + 1
		}
		pre = c
	}
	return 0, pre, false
}

// getFormatOrQuestionMarkPos both find %x and ? and %%s%
func (di *DbInput) getFormatOrQuestionMarkPos(index int) (int, rune, bool){
	count := 0
	var pre rune
	found := false
	for i, c := range *di.format{
		if found{
			if c == '%'{
				found = false // like %%x%
			}else{
				return i - 1, c, true
			}
		}
		if c == '?'{
			if count == index{
				return i, c, true
			}
			count = count + 1
		} else if i > 0 && c == '%' && pre != '\\' {
			if count == index{
				//return i, pre, true
				found = true
			}
			count = count + 1
		}
		pre = c
	}
	return 0, pre, false
}

func (di *DbInput) appendParas(paras [] *functionPara) {
	for _, p1 := range paras{
		found := false
		for j, p2 := range di.paras{
			if p2 == nil{
				di.paras[j] = p1
				found = true
				break
			}
		}
		if !found {
			di.paras = append(di.paras, p1)
		}
	}
}

func (di *DbInput) concat(input *DbInput)*DbInput{
	// merge format
	s := (*di).format
	if (*di).format != nil{
		s = &(*(*di).format)
	}
	if (*input).format != nil{
		if s == nil{
			s = &(*(*input).format)
		}else{
			*s = *s + *(*input).format
		}
	}
	(*di).format = s
	di.paras = append(di.paras, input.paras...) 
	// merge paras
	return di
}

func (di *DbInput) merge() *DbInput{
	r := &DbInput{}
	n := (*di).next
	for{
		if n == nil || n == di{
			break
		}
		r = (*r).concat(n)
		n = (*n).next
	}
	return r
}

func (di *DbInput) addFormat(input *DbInput) *DbInput{
	di.format = input.format
	di.paras = append(input.paras, di.paras...)
	return di
}

func (di *DbInput) addParameter(input *DbInput) *DbInput{
	if (*input).next == nil{
		return di.addParameterSingle(input)
	}else{
		n := (*input).next
		for{
			if n == input{
				break
			}
			di = di.addParameterSingle(n)
			n = (*n).next
		}
		return di
	}
}

func (di *DbInput) addDbcallParameter(input *DbInput) *DbInput{
	if (*input).next == nil{
		return di.addDbCallParameterSingle(input)
	}else{
		n := (*input).next
		for{
			if n == input{
				break
			}
			di = di.addDbCallParameterSingle(n)
			n = (*n).next
		}
		return di
	}
}

// addParameterSingle for sprintf
func (di *DbInput) addParameterSingle(input *DbInput) *DbInput{
	if len(input.paras) > 0 || input.format != nil{
		index := len(di.paras)
		//fmt.Println("format ", *di.format, " ", index)
		if pos, preChar, ok := di.getFormatPos(index); ok{
			//fmt.Println("hexinmin add with format 2 ", index, " ",pos, " ", preChar)
			if preChar == 's'{
				// merge format
				if input.format == nil{
					// only parameter
					if len(input.paras) > 1{
						panic(*di)
					}
					di.paras = append(di.paras, input.paras[0])
				} else {
					// merge format
					s := (*di.format)[:pos] + *input.format + (*di.format)[pos + 2:]
					di.format = &s
					di.paras = append(di.paras, input.paras...)
				}
			} else {
				// format not change
				di.paras = append(di.paras, &functionPara{})
			}
		} else {
			//if len(input.paras) > 1{
				panic("di:" + di.String() + "input:" + input.String())
			//}
			//di.paras = append(di.paras, input.paras[0])
		}
	}else{
		//panic(1)
	}

	return di
}

// addDbCallParameterSingle for dbcall
func (di *DbInput) addDbCallParameterSingle(input *DbInput) *DbInput{
	if len(input.paras) > 0 || input.format != nil{
		index := len(di.paras)
		//fmt.Println("format ", *di.format, " ", index)
		if pos, preChar, ok := di.getFormatOrQuestionMarkPos(index); ok{
			//fmt.Println("hexinmin add with format 2 ", index, " ",pos, " ", preChar)
			if preChar == 's'{
				// merge format
				if input.format == nil{
					// only parameter
					if len(input.paras) > 1{
						panic(3)
					}
					di.paras = append(di.paras, input.paras[0])
				} else {
					// merge format
					s := (*di.format)[:pos] + *input.format + (*di.format)[pos + 2:]
					di.format = &s
					di.paras = append(di.paras, input.paras...)
				}
			}else if preChar == '?'{
				// neglect input format, can not unfold
				p := &functionPara{}
				if len(input.paras) > 0{
					p = input.paras[0]
					for i, ip := range input.paras{
						if i > 0{
							p.conflate(ip)
						}
					}
				}
				//fmt.Println("append parameter :", *p, " ", (*input).String())
				di.paras = append(di.paras, p)
			}else{
				// format not change
				di.paras = append(di.paras, &functionPara{})
			}
		} else {
			//if len(input.paras) > 1{
				panic("di:" + di.String() + "input:" + input.String())
			//}
			//di.paras = append(di.paras, input.paras[0])
		}
	}else{
		//panic(1)
	}
	return di
}

func (di *DbInput) reportError(fun string, paras [] functionPara)string{
	s := ""
	if di.Empty(){
		return s
	}

	for i := 0; ; i ++{
		if _, c, ok := di.getFormatOrQuestionMarkPos(i); ok{
			if c == 's' && i < len(di.paras){
				for _, p := range(paras){
					if di.paras[i] != nil && di.paras[i].pName == p.pName{
						s = fun + " exist sql injection" 
						return s
					}
				}
			}
			// check
		}else{
			break
		}

	}
	return s
}

type ExtraceName struct {
	result  string
	selectFlag bool
	caseStack   *list.List
}

func NewExtraceName() *ExtraceName{
	return &ExtraceName{
		selectFlag : false,
		caseStack:   list.New(),}
}

func (en *ExtraceName) Visit(n ast.Node) ast.Visitor {
	if n == nil{
		en.upDateStateAfterPop()
		return nil
	} else {
		en.caseStack.PushBack(n)
		switch t:= n.(type){
		case *ast.Ident:{
				en.result = en.result + t.Name
			}
		case *ast.StarExpr:{
				en.result = en.result + "*"
			}
		case *ast.ArrayType:{
				en.result = en.result + "[]"
			}
		case *ast.SelectorExpr:{
			en.selectFlag = true
		}
		}
	}
	return en
}

func (en *ExtraceName) upDateStateAfterPop(){
	e := en.caseStack.Back()
	en.caseStack.Remove(e)
	if last := en.caseStack.Back(); last != nil{
		if _, ok := last.Value.(*ast.SelectorExpr); ok{
			if en.selectFlag{
				en.result = en.result + "."
				en.selectFlag = false
			}
		}
	}
}

type Analyzer struct {
	ignoreNosec bool
	catchError bool
	caseStack   *list.List
	parameters  [] functionPara
	curParaName   string
	curParaType   string
	state       StateMentAnalysis
	curFunName    string
	logger      *log.Logger
	dbCallPara   map[string]string
	allPossibleInput map[string] *DbInput
	result [] string
}

func (si *Analyzer) isFunctionParaName() bool {
	last := si.caseStack.Back() // type is []*ast.Ident
	if last = last.Prev(); last != nil{
		switch last.Value.(type){
		case *ast.FieldList:
			if last = last.Prev(); last != nil{
				switch last.Value.(type){
				case *ast.FuncType:
					if last = last.Prev(); last != nil{
						switch last.Value.(type){
						case *ast.FuncDecl:
							return true
						}
					}
				}
			}
		}
	}	
	return false
}

func (si *Analyzer) isFunctionBlockStmt() bool {
	last := si.caseStack.Back() // type is *ast.BlockStmt
	if last = last.Prev(); last != nil{
		switch last.Value.(type){
		case *ast.FuncDecl:{
				return true
			}	
		default:
			
		}
	}
	return false
}

func (si *Analyzer) isSprintfCall(n ast.Node) bool{
	switch node:= n.(type){
	case *ast.CallExpr:
		//switch f := node.Fun.(type){
		switch node.Fun.(type){
		case *ast.SelectorExpr:
			return true
			//if f.Sel.Name == "Sprintf"{
			//	return true
			//}
		}
	}
	return false
}

func (si *Analyzer) ChangeState(s StateMentAnalysis){
	//fmt.Println("state change from  " , si.state, " to ", s)
	si.state = s
}

// todo : delete
func getParaType(f ast.Field) string {
	result := ""
	switch t:= f.Type.(type){
	case *ast.Ident:{
			return t.Name
		}
	case *ast.StarExpr:{
			result = "*"
			switch seType:= t.X.(type){
			case *ast.SelectorExpr:
				switch xt:= seType.X.(type){
				case *ast.Ident:
					result = result + xt.Name
				default:
					panic(xt)
				}
				result = result + "." + seType.Sel.Name
			}
		}
	case *ast.SelectorExpr:{
		switch xt:= t.X.(type){
		case *ast.Ident:
			result = result + xt.Name
		default:
			panic(xt)
		}
		result = result + "." + t.Sel.Name
	}
	case *ast.ArrayType:{
		switch Elt := t.Elt.(type){
		case *ast.Ident:{
			result = result + "[]" + Elt.Name
		}
	default:
			panic(Elt)
		}
	}
	default:
		panic(t)
	}

	return result
} 

func (si *Analyzer) upDateStateAfterPop(){
	lastElement := si.caseStack.Back()
	switch lastElement.Value.(type){
	case *ast.BlockStmt:{
			if si.isFunctionBlockStmt() && 
				si.state == StateMentAnalysis_FUNCTION_BODY{
				si.ChangeState(StateMentAnalysis_FUNCTION)
			}
		}
	case *ast.FuncDecl:{
			if si.state == StateMentAnalysis_FUNCTION{
				si.parameters = nil
				si.curFunName = ""
				si.allPossibleInput = make(map[string] *DbInput)
				si.ChangeState(StateMentAnalysis_START)
			}
		}
	}
	si.caseStack.Remove(lastElement)
}

func isStringFormat(str string, pos int) bool{
	count := 0
	for i:= 0; i + 1 < len(str); i++{
		if str[i] == '%'{
			count++
		}

		if count == pos{
			if str[i] == 's'{
				return true
			}
		}
	}

	return false
}

func (si *Analyzer) isFunctionParameters(v string) bool{
	for _, para := range si.parameters{
		if para.pName == v{
			return true
		}
	}

	return false
}

func (si *Analyzer) isDbCallFunction() bool{
	for _, para := range si.parameters{
		if para.pType == "*sqlx.DB" ||
			para.pType == "kitSql.DbInterface" {
			return true
		}
	}
	return false
}

func (si *Analyzer) getDbInputFromRhs(n ast.Node) *DbInput {
	di := &DbInput{}
	// fmt.printf
	if callexp, ok := n.(*ast.CallExpr); ok {
		if f, ok := callexp.Fun.(*ast.SelectorExpr); ok{
			if x, ok := f.X.(*ast.Ident); ok {
				if x.Name == "fmt" && f.Sel.Name == "Sprintf"{
					// deal format
					for i, arg := range callexp.Args{
						if i == 0{
							// ad format
							addFormat := si.getDbInputFromRhs(arg)
							di = di.addFormat(addFormat)
						} else {
							// add parameter
							addParameter := si.getDbInputFromRhs(arg)
							di = di.addParameter(addParameter)
						}
					}
				} else if x.Name == "strings" && f.Sel.Name == "Join"{
					di = si.getDbInputFromRhs(callexp.Args[0])
					join := si.getDbInputFromRhs(callexp.Args[1])
					(*di).likeStringJoin(join)
					di = di.merge()
				}
			}
		} else if f, ok := callexp.Fun.(*ast.Ident); ok{
			if f.Name == "append"{
				for i, arg := range callexp.Args{
					if i == 0{
						di = si.getDbInputFromRhs(arg)
					} else {
						new := si.getDbInputFromRhs(arg)
						(*di).appendTail(new)
					}
				}
			}
		}
	}else if v, ok := n.(*ast.BinaryExpr); ok {
		if v.Op == token.ADD{
			X := si.getDbInputFromRhs(v.X)
			Y := si.getDbInputFromRhs(v.Y)
			(*X).updateForAdd()
			(*Y).updateForAdd()
			di = (*X).concat(Y)
		}
	}else if v, ok := n.(*ast.BasicLit); ok { // plain text
		s := v.Value
		s = s[1:len(s)-1]
		return &DbInput{
			format: &s,
		}
	}else if v, ok := n.(*ast.Ident); ok{ // *ast.Ident
		if k, ok := si.allPossibleInput[v.Name]; ok{
			return k
		} else {
			return &DbInput{
				paras: [] *functionPara {&functionPara{pName:v.Name,},},
			}
		}
	}else if v, ok := n.(*ast.CompositeLit); ok{
		if Type, ok := v.Type.(*ast.ArrayType); ok{
			switch Elt := Type.Elt.(type) {
				case *ast.Ident:{
					// []string{}
					if Elt.Name == "string"{
						di = &DbInput{}
						(*di).next = di
					}	
				}
				case *ast.InterfaceType:{
					// []interface{}{}
					if !Elt.Incomplete{
						di = &DbInput{}
						(*di).next = di
					}
				}
			}
		}
	}else{
		en := NewExtraceName()
		ast.Walk(en, n)
		if en.result != ""{
			di.paras = append(di.paras, &functionPara{pName:en.result,})
		}
	}
	// []interface{}{}
	
	// to do else
	return di
}

func (si *Analyzer) AddDbCallPara(n string, t string){
	switch t{
	case "*sqlx.DB", "*sqlx.Tx":
		{
			if _, ok := si.dbCallPara[n]; !ok{
				si.dbCallPara[n] = t
			}
		}
	}
}

func (si *Analyzer) isDbInterfaceCall(n *ast.CallExpr) (string, string, bool) {
	if f, ok := n.Fun.(*ast.SelectorExpr); ok{
		if x, ok := f.X.(*ast.Ident); ok{
			if v, ok := si.dbCallPara[x.Name]; ok{
				return v, f.Sel.Name, true
			}
		}
	}
	return "", "", false
}

func (si *Analyzer) analyzeDbCall(di *DbInput, ce *ast.CallExpr, index int){
	for i, arg := range ce.Args{
		if i == index{
			addFormat := si.getDbInputFromRhs(arg)
			di = (*di).addFormat(addFormat)
		} else if i > index {
			addPara := si.getDbInputFromRhs(arg)
			di = (*di).addDbcallParameter(addPara)
		}
	}
}

func (si *Analyzer) checkDbCall(node *ast.CallExpr, iType string, fName string){
	di := &DbInput{}
	switch iType{
	case "*sqlx.DB":{
		switch fName{
			case "Get","Select":{
					si.analyzeDbCall(di, node, 1)
				}
			}
		}
	case "*sqlx.Tx":{
		switch fName{
			case "Exec":{
					si.analyzeDbCall(di, node, 0)
				}
			case "Get":{
					si.analyzeDbCall(di, node, 1)
				}
			}
		}
	}	
	
	//fmt.Println("final di is ")
	fmt.Println(di.toString())
	s := di.reportError(si.curFunName, si.parameters)
	if s != ""{
		fmt.Println(s)
		si.result = append(si.result, s)
	}
}

func (si *Analyzer) Visit(n ast.Node) ast.Visitor {
	defer func(){ 
        //fmt.Println("catch error")
        if err:=recover();err!=nil{
			fmt.Println(err) 
			si.catchError = true
        }
    }()

	if n == nil{
		si.upDateStateAfterPop()
		//fmt.Printf("pop len is %d\n", si.caseStack.Len())
		return nil
	} else {
		si.caseStack.PushBack(n)
		//fmt.Printf("push len is %d\n", si.caseStack.Len())
		switch node:= n.(type){
			case *ast.Field:
				/*
				if si.isFunctionParaName(){
					if len(node.Names) > 0{
						//fmt.Printf("%s %s\n", node.Names[0].Name, getParaType(*node))
						si.parameters = append(si.parameters, 
							functionPara{pName:node.Names[0].Name, pType:getParaType(*node)})
						si.AddDbCallPara(node.Names[0].Name, getParaType(*node))
					}
				}*/
			case *ast.FuncDecl:
				if	si.state == StateMentAnalysis_START{
					si.curFunName = node.Name.Name
					si.catchError = false
					fmt.Println("check " + si.curFunName)
					si.ChangeState(StateMentAnalysis_FUNCTION)
				}
				for _, para := range node.Type.Params.List{
					en := NewExtraceName()
					ast.Walk(en, para.Type)
					//fmt.Println("para type:", en.result)
					si.parameters = append(si.parameters, 
						functionPara{pName:para.Names[0].Name, pType:en.result})
					si.AddDbCallPara(para.Names[0].Name, en.result)
				}
			case *ast.BlockStmt:{
				if	si.state == StateMentAnalysis_FUNCTION{
					si.ChangeState(StateMentAnalysis_FUNCTION_BODY)
				}
			}
			case *ast.CallExpr:{
				//fmt.Printf("call expr %d\n", si.state )
				if si.state == StateMentAnalysis_FUNCTION_BODY && !si.catchError{

					/*
					if si.isSprintfCall(n){
						//fmt.Printf("is sprintf call\n")
						if len(node.Args) >= 2{
							switch format := node.Args[0].(type){
							case *ast.BasicLit:
								for i, para := range(node.Args){
									if i > 0{
										switch p := para.(type){
										case *ast.Ident:
											if isStringFormat(format.Value, i) && si.isFunctionParameters(p.Name) && si.isDbCallFunction(){
												fmt.Printf("sql injectiton:%s\n", si.curFunName)
											}
										}
									}
								}
							}
						}
					}*/
					
					if iType, fName, ok := si.isDbInterfaceCall(node); ok{
						si.checkDbCall(node, iType, fName)
					}
					
				}
			}
			case *ast.AssignStmt:{
				if si.state == StateMentAnalysis_FUNCTION_BODY && !si.catchError{
					// del right
					
					if node.Tok == token.ADD_ASSIGN{
						dbInput := si.getDbInputFromRhs(node.Rhs[0])
						if !dbInput.Empty(){
							if v, ok := node.Lhs[0].(*ast.Ident); ok{
								left := si.getDbInputFromRhs(node.Lhs[0])
								if !left.Empty(){
									dbInput.updateForAdd()
									left = (left).concat(dbInput)
									si.allPossibleInput[v.Name] = left
									//fmt.Println("allPossibleInput add += ", v.Name, ":", left)
								}
							}
						}
					} else {
						dbInput := si.getDbInputFromRhs(node.Rhs[0])
						if !dbInput.Empty(){
							if v, ok := node.Lhs[0].(*ast.Ident); ok{
								si.allPossibleInput[v.Name] = dbInput
								//fmt.Println("allPossibleInput add ", v.Name, ":", dbInput)
							}
						}
					}

				}
			}
		}
		return si
	}
}

/*
func (si *Analyzer) check(pkg *packages.Package) {
	for _, file := range pkg.Syntax {
		//si.logger.Println("Checking file:", pkg.Fset.File(file.Pos()).Name())
		ast.Walk(si, file)
	}
}
*/



type SqlInject struct{}

func (SqlInject) Name() string {
	return "sqlinject"
}

func (SqlInject) Desc() string {
	return "analyze sql injection"
}

func (lint SqlInject) Run(ctx context.Context, lintCtx *linter.Context) ([]result.Issue, error) {
	
	si  := &Analyzer{
		logger:	log.New(os.Stderr, "[sqlinj]", log.LstdFlags),
		caseStack:   list.New(),
		parameters:       make([]functionPara,1),
		state:       StateMentAnalysis_START,
		//result :          make([]string, 1),
		catchError : false,
		allPossibleInput: make(map[string]*DbInput),
		dbCallPara:       make(map[string]string),
	}

	for _, pkg := range lintCtx.Program.InitialPackages(){
		for _, file := range pkg.Files{
			ast.Walk(si, file)
		}
	}

	for _, r := range si.result{
		lintCtx.Log.Warnf(r)
	}

	if len(si.result) == 0{
		return nil, nil
	}

	res := make([]result.Issue, 0, len(si.result))
	for _, r := range si.result {
		res = append(res, result.Issue{
			Text:       r,
		})
	}

	return res, nil
/*
	gasConfig := gosec.NewConfig()
	enabledRules := rules.Generate()
	logger := log.New(ioutil.Discard, "", 0)
	analyzer := gosec.NewAnalyzer(gasConfig, logger)
	analyzer.LoadRules(enabledRules.Builders())

	analyzer.ProcessProgram(lintCtx.Program)
	issues, _ := analyzer.Report()
	if len(issues) == 0 {
		return nil, nil
	}

	res := make([]result.Issue, 0, len(issues))
	for _, i := range issues {
		text := fmt.Sprintf("%s: %s", i.RuleID, i.What) // TODO: use severity and confidence
		var r *result.Range
		line, err := strconv.Atoi(i.Line)
		if err != nil {
			r = &result.Range{}
			if n, rerr := fmt.Sscanf(i.Line, "%d-%d", &r.From, &r.To); rerr != nil || n != 2 {
				lintCtx.Log.Warnf("Can't convert gosec line number %q of %v to int: %s", i.Line, i, err)
				continue
			}
			line = r.From
		}

		res = append(res, result.Issue{
			Pos: token.Position{
				Filename: i.File,
				Line:     line,
			},
			Text:       text,
			LineRange:  r,
			FromLinter: lint.Name(),
		})
	}

	return res, nil
*/
}

