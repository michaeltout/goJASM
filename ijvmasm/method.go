package ijvmasm

import (
	"errors"
	"github.com/sirupsen/logrus"
	"io"
	"strings"

	"github.com/BlackNovaTech/gojasm/opconf"
)

// Method represents a single IJVM method context
type Method struct {
	name         string
	vars         []string
	instructions []*Instruction
	labels       []*Label
	end          string

	bytes    uint32
	numparam int

	N uint32
	B uint32

	wide bool
}

// Label represents a single label in JAS.
// Is used to calculate GOTO and friends' offsets
type Label struct {
	Name string
	N    uint32
	B    uint32
}

// NewMethod returns a new Method based on the given method declaration and line number.
// Parameters are parsed from the string and added to the parameter list.
func NewMethod(nameParam string, N uint32) (*Method, error) {
	var startbyte uint32
	end := JASMethodEnd
	var params []string
	name := "main"
	if nameParam == "main" {
		end = JASMainEnd
	} else {
		if !strings.ContainsRune(nameParam, '(') {
			return nil, errors.New("invalid method declaration. Missing opening parenthesis")
		}
		rawName, paramstr := splitLink(nameParam, "(")
		name = strings.TrimSpace(rawName)
		if !strings.ContainsRune(paramstr, ')') {
			return nil, errors.New("invalid method declaration. Missing closing parenthesis")
		}
		paramstr, junk := splitLink(paramstr, ")")
		if strings.TrimSpace(junk) != "" {
			return nil, errors.New("invalid method declaration. Characters remaining after parameter list")
		}

		paramlst := strings.Split(paramstr, ",")
		// BS empty parameter list
		if paramstr == "" {
			paramlst = []string{}
		}

		params = make([]string, len(paramlst)+1)
		params[0] = "LINK PTR"
		for i, p := range paramlst {
			params[i+1] = strings.TrimSpace(p)
		}

		startbyte = 4
	}

	return &Method{
		name:         name,
		vars:         params,
		numparam:     len(params),
		instructions: make([]*Instruction, 0),
		labels:       make([]*Label, 0),
		end:          end,
		N:            N,
		bytes:        startbyte,
		B:            startbyte,
	}, nil

}

// AppendInst appends a single instruction to the instruction stream.
func (m *Method) AppendInst(inst *Instruction) {
	m.instructions = append(m.instructions, inst)
}

// VarIndex fetches the variable index of the given variable name.
// Returns ok iff variable is found.
func (m *Method) VarIndex(str string) (int, bool) {
	for i, p := range m.vars {
		if p == str {
			return i, true
		}
	}
	return -1, false
}

// Finds label for linking
func (m *Method) findLabel(name string) (bool, int, *Label) {
	for i, label := range m.labels {
		if label.Name == name {
			return true, i, label
		}
	}
	return false, -1, nil
}

// LinkLabels iterates over the instruction stream and replaces every GOTO&friends label with the correct
// offset.
func (m *Method) LinkLabels() (ok bool) {
	ok = true
	for _, inst := range m.instructions {
		if !inst.linkLabel {
			continue
		}
		for j, argType := range inst.op.Args {
			if argType == opconf.ArgLabel {
				found, _, lbl := m.findLabel(inst.label)
				if !found {
					logrus.Errorf("[.%s] Undefined label `%s` at line %d", m.name, inst.label, inst.N)
					ok = false
				}
				inst.params[j] = int(lbl.B) - int(inst.B)
				logrus.Debugf("[.%s] Linking label, line %d: @%d -> %s@%d, offset = %d",
					m.name, inst.N, inst.B, lbl.Name, lbl.B, inst.params[j])
			}
		}
	}
	return
}

// LinkMethods iterates over the instruction stream and replaces every INVOKEVIRTUAL&friends method argument with the correct
// constant index.
func (m *Method) LinkMethods(asm *Assembler) (ok bool) {
	ok = true
	for _, inst := range m.instructions {
		if !inst.linkMethod {
			continue
		}
		for j, argType := range inst.op.Args {
			if argType == opconf.ArgMethod {
				found, idx, mtd := asm.findConstant(inst.label)
				if !found {
					logrus.Errorf("[.%s] Undefined method `%s` at line %d", m.name, inst.label, inst.N)
					ok = false
				}
				inst.params[j] = idx
				logrus.Debugf("[.%s] Linking method, line %d: %s -> %d",
					m.name, inst.N, mtd.Name, inst.params[j])
			}
		}
	}
	return
}

// Generate the Method's corresponding IJVM binary code
func (m *Method) Generate(out io.Writer) {
	for _, inst := range m.instructions {
		mustWrite(out, inst.op.Opcode)
		for i, arg := range inst.op.Args {
			switch arg {
			case opconf.ArgByte:
				mustWrite(out, uint8(inst.params[i]))
			case opconf.ArgVar:
				if inst.wide {
					mustWrite(out, uint16(inst.params[i]))
				} else {
					mustWrite(out, uint8(inst.params[i]))
				}
			case opconf.ArgLabel:
				fallthrough
			case opconf.ArgConst:
				fallthrough
			case opconf.ArgMethod:
				mustWrite(out, uint16(inst.params[i]))

			default:
				panic("Unimplemented")
			}
		}
	}
}
