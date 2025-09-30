package main

import (
	"flag"
	"fmt"
	"go/format"
	"io"
	"maps"
	"math"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

type FSMDefinition struct {
	Name        string
	Imports     []string
	PackageName string
	UseSLog     bool
	Events      map[string]FSMEventDefinition
}

type FSMEventDefinition struct {
	Source      []string
	Destination string
	Params      []FSMEventParams
}

type FSMEventParams struct {
	Name string
	Type string
}

type _States []string

func _GetStates(def FSMDefinition) _States {
	stateSet := map[string]bool{}

	for _, event := range def.Events {
		stateSet[event.Destination] = true
		for _, state := range event.Source {
			stateSet[state] = true
		}
	}

	states := slices.AppendSeq([]string{}, maps.Keys(stateSet))
	slices.Sort(states)
	return states
}

func _GetStateName(s string) string {
	return "STATE_" + strings.ToUpper(s)
}

func _GetNeededUintSize(count int) string {
	stateEnumType := "uint8"

	if count > math.MaxUint8 {
		stateEnumType = "uint16"
	}
	if count > math.MaxUint16 {
		stateEnumType = "uint32"
	}
	if count > math.MaxUint32 {
		stateEnumType = "uint64"
	}
	return stateEnumType
}

func ParseTOML(r io.Reader) (FSMDefinition, error) {
	fsm := FSMDefinition{}
	_, err := toml.NewDecoder(r).Decode(&fsm)
	return fsm, err
}

func BuildText(definition FSMDefinition) string {
	builder := strings.Builder{}

	states := _GetStates(definition)

	GenerateHeader(&builder, definition)
	GenerateStateDefinition(&builder, definition, states)
	GenerateInitalizer(&builder, definition)
	GenerateFSMDefinition(&builder, definition)
	GenerateLookup(&builder, states)

	i := 0
	for eventName, event := range definition.Events {
		GenerateFSMEvent(&builder, definition, states, i, eventName, event)
		i++
	}
	return builder.String()
}

func GenerateHeader(builder *strings.Builder, definition FSMDefinition) {
	fmt.Fprintf(
		builder,
		HEADER,
		definition.PackageName,
	)

	if definition.UseSLog {
		builder.WriteString("\"log/slog\"\n")
	}

	for _, imprt := range definition.Imports {
		fmt.Fprintf(builder, "\"%v\"\n", imprt)
	}

	builder.WriteRune(')')
}

func GenerateStateDefinition(builder *strings.Builder, definition FSMDefinition, states _States) {

	stateEnumType := _GetNeededUintSize(len(states))

	state0 := states[0]

	fmt.Fprintf(
		builder,
		STATES_DEF,
		stateEnumType,
		_GetStateName(state0),
	)

	for i, state := range states {
		if i == 0 {
			continue
		}
		builder.WriteString(_GetStateName(state))
		builder.WriteRune('\n')
	}

	builder.WriteString("\n)")

}

func GenerateFSMDefinition(builder *strings.Builder, definition FSMDefinition) {
	fmt.Fprintf(
		builder,
		FSM_DEF,
		definition.Name,
		_GetNeededUintSize(len(definition.Events)),
	)
}

func GenerateFSMEvent(builder *strings.Builder, definition FSMDefinition, states _States, index int, eventName string, event FSMEventDefinition) {

	validSrcs := []string{}
	signature := []string{}
	callParams := []string{}

	for _, src := range event.Source {
		validSrcs = append(validSrcs, _GetStateName(src))
	}

	logging := ""
	lsb := strings.Builder{}

	if definition.UseSLog {
		// lsb.WriteString("slog.With(\"\", ")
		fmt.Fprintf(&lsb, "slog.With(\"Start State\", fsm.State,")
	}

	for _, param := range event.Params {
		signature = append(signature, fmt.Sprintf("%v %v", param.Name, param.Type))
		callParams = append(callParams, param.Name)
		fmt.Fprintf(&lsb, "\"%v\", %v,", param.Name, param.Name)
	}

	if definition.UseSLog {
		// lsb.WriteString(").Info(\"User has transitioned to %v\")")
		fmt.Fprintf(
			&lsb,
			").Info(\"User has transitioned to %v\")",
			_GetStateName(eventName),
		)
		logging = lsb.String()
	}

	ti := []any{}
	ti = append(ti, eventName)
	ti = append(ti, strings.Join(signature, ","))
	ti = append(ti, definition.Name)
	ti = append(ti, eventName)
	ti = append(ti, strings.Join(signature, ","))
	ti = append(ti, strings.Join(validSrcs, ","))
	ti = append(ti, eventName)
	ti = append(ti, "%v")
	ti = append(ti, logging)
	ti = append(ti, index)
	ti = append(ti, eventName)
	ti = append(ti, strings.Join(callParams, ","))
	ti = append(ti, _GetStateName(event.Destination))
	ti = append(ti, definition.Name)
	ti = append(ti, eventName)
	ti = append(ti, eventName)
	ti = append(ti, index)

	fmt.Fprintf(
		builder,
		EVENT,
		ti...,
	)
}

func GenerateInitalizer(builder *strings.Builder, definition FSMDefinition) {
	fmt.Fprintf(
		builder,
		INIT,
		definition.Name,
		definition.Name,
		_GetNeededUintSize(len(definition.Events)),
	)
}

func GenerateLookup(builder *strings.Builder, states _States) {
	builder.WriteString(LOOKUP_DEF)
	for i, state := range states {
		fmt.Fprintf(builder, "%v:\"%v\",\n", i, state)
	}
	builder.WriteRune('}')
}

var (
	TARGET_FILE string
	DEST_FILE   string
)

func init() {
	flag.StringVar(&TARGET_FILE, "target-file", "fsm.toml", "FSM definition to generate from")
	flag.StringVar(&DEST_FILE, "dest-file", "fsm_GEN.go", "File to write generated code too")
	flag.Parse()
}

func main() {
	f, err := os.Open(TARGET_FILE)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	fsm, err := ParseTOML(f)
	if err != nil {
		panic(err)
	}

	generatedCode := BuildText(fsm)
	formatted, err := format.Source([]byte(generatedCode))
	if err != nil {
		panic(err)
	}

	if err = os.WriteFile(DEST_FILE, formatted, os.ModePerm); err != nil {
		panic(err)
	}
}
