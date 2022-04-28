package main

import (
    "fmt"
    "strings"
)

// to be formatterd by 'fmt.Sprintf'
var mips_code_base string = `.data
%s
.text
    main:
%s
        move $2, $0
        j $31
`

// filters out all empty strings ("") from a given input
func filter_out_blank(array []string) (ret []string) {
    for _, s := range array {
        if s != "" {
            ret = append(ret, s)
        }
    }
    return
}

// the entire program; a container for other nodes
type Program struct {
    nodes []interface{}
}

// an identifier
type Ident struct {
    name string
}

// an arithmetic operation; supports:
// a add b
// a sub b
// a mul b
// a div b
type ArithmeticOp struct {
    left  interface{}
    op    string
    right interface{}
}

// an assignment of the form:
// a = b
type Assignment struct {
    name  string
    value interface{}
}

// a basic integer
type Integer struct {
    value string
}

// a basic string
type String struct {
    value string
}

// an instruction of the form (where (a, b, c) are the arguments):
// opcode a, b, c
type Instruction struct {
    opcode string
    args   []string
}

// the code generator
type MIPSBackend struct {
    temp_registers [10]string
    access_loc     map[string]string
    name_offset    uint
    temp_reg_id    uint
    data_temp_name uint
    stack          []string
    data_section   string
    main_section   []Instruction
}

// 'MIPSBackend' constructor
func new_mips_backend(ast interface{}) MIPSBackend {
    var backend MIPSBackend = MIPSBackend{
        [10]string{
            "$t9", "$t8", "$t7", "$t6", "$t5",
            "$t4", "$t3", "$t2", "$t1", "$t0",
        },
        map[string]string{},
        4,
        0,
        1,
        []string{},
        "",
        []Instruction{},
    }
    // generate the code
    backend.codegen(ast)
    return backend
}

// emit an instruction
func (backend *MIPSBackend) __emit_main(params ...string) {
    if len(params) > 4 {
        panic("too many arguments supplied to '__emit_main'")
    }
    backend.main_section = append(backend.main_section, Instruction{params[0],
        []string{params[1], params[2], params[3]}})
}

// emit to the data section
func (backend *MIPSBackend) __emit_data(data string) {
    backend.data_section += fmt.Sprintf("    %s\n", data)
}

// create a new temporary register
// NOTE: this doesn't to see if we have used up
// all the temporary registers
// TODO: implement the register allocation algorithm
func (backend *MIPSBackend) __temp_register() string {
    backend.temp_reg_id++
    return fmt.Sprintf("$t%d", backend.temp_reg_id-1)
}

// returns the final mips code
func (backend *MIPSBackend) assemble() string {
    var main_section string
    for _, instruction := range backend.main_section {
        var args string = strings.Join(filter_out_blank(instruction.args), ",")
        main_section += fmt.Sprintf("        %s %s\n", instruction.opcode, args)
    }
    return fmt.Sprintf(mips_code_base, backend.data_section, main_section)
}

// a recursive function that generates code
// for a given ast
func (backend *MIPSBackend) codegen(__node interface{}) {
    switch node := __node.(type) {
    case Program:
        for _, item := range node.nodes {
            backend.codegen(item)
        }
    case ArithmeticOp:
        backend.arithmetic_op(&node)
    case Assignment:
        backend.assignment(&node)
    case Ident:
        backend.ident(&node)
    case Integer:
        backend._integer(&node)
    case String:
        backend._string(&node)
    }
}

// an arithmetic operation; converts:
// a + b
// =>
// <code for a>
// <code for b>
// op $t1, $t0, $t1
// such that $t0 is a's register, and $t1 is b's
func (backend *MIPSBackend) arithmetic_op(node *ArithmeticOp) {
    backend.codegen(node.left)
    backend.codegen(node.right)
    var (
        left_register  string
        right_register string
        i              int = len(backend.stack) - 1
    )
    // we have to pop the right register from the stack,
    // then the left register because the right hand-side
    // was generated last
    right_register, backend.stack = backend.stack[i], append(backend.stack[:i], backend.stack[0:]...)
    left_register, backend.stack = backend.stack[i], append(backend.stack[:i], backend.stack[0:]...)
    // store the value in the right register
    backend.__emit_main(node.op, right_register, left_register, right_register)
    // push the right register onto the stack
    backend.stack = append(backend.stack, right_register)
}

// an assignment; converts:
// a = b
// =>
// <code for b>
// sw $t0, -4($sp)
// such that $t0 is b's register and -4 is the
// current offset from the stack pointer
func (backend *MIPSBackend) assignment(node *Assignment) {
    backend.codegen(node.value)
    backend.access_loc[node.name] = fmt.Sprintf("-%d($sp)", backend.name_offset)
    // increment the offset by 4 (the word size)
    backend.name_offset += 4
    var (
        value_register string
        i              int = len(backend.stack) - 1
    )
    // pop the stack to get the register the value is stored in
    value_register, backend.stack = backend.stack[i], append(backend.stack[:i], backend.stack[0:]...)
    backend.__emit_main("sw", value_register, backend.access_loc[node.name], "")
}

// emits:
// lw $t0, -4($sp)
// such that $t0 is the first temporary register it could
// get, and -4 is the offset from the stack pointer
func (backend *MIPSBackend) ident(node *Ident) {
    // get a new temporary register
    var temp_register string = backend.__temp_register()
    // push the register onto the stack
    backend.stack = append(backend.stack, temp_register)
    backend.__emit_main("lw", temp_register, backend.access_loc[node.name], "")
}

// emits:
// li $t0, 123
// such that $t0 is the first temporary register it could
// get, and 123 is the value of the integer
func (backend *MIPSBackend) _integer(node *Integer) {
    // get a new temporary register
    var temp_register string = backend.__temp_register()
    // push the register onto the stack
    backend.stack = append(backend.stack, temp_register)
    backend.__emit_main("li", temp_register, node.value, "")
}

// emits:
// string1: .asciiz "abc"
// in the data section, and:
// la $t0, string1
// such that $t0 is the first temporary register it could
// get, and "abc" is the value of the string
func (backend *MIPSBackend) _string(node *String) {
    // get a new temporary register
    var temp_register string = backend.__temp_register()
    // push the register onto the stack
    backend.stack = append(backend.stack, temp_register)
    // we have to store the string in the data section
    backend.__emit_data(
        fmt.Sprintf("string%d: .asciiz \"%s\"", backend.data_temp_name, node.value))
    backend.__emit_main("la", temp_register, fmt.Sprintf("string%d", backend.data_temp_name), "")
    backend.data_temp_name++
}

func main() {
    // ast is equivlent to:
    // abc = 123 + (321 - 123)
    var ast Program = Program{
        []interface{}{
            Assignment{
                "foo",
                ArithmeticOp{
                    Integer{"123"},
                    "add",
                    ArithmeticOp{
                        Integer{"321"},
                        "sub",
                        Integer{"123"},
                    },
                },
            },
            Assignment{
                "bar",
                String{"foobar"},
            },
            Assignment{
                "baz",
                Ident{"bar"},
            },
        },
    }
    var backend MIPSBackend = new_mips_backend(ast)
    fmt.Println(backend.assemble())
    // output MIPS assembly is:

    //  .data
    //     string1: .asciiz "foobar"
    //
    // .text
    //     main:
    //         li $t0,123
    //         li $t1,321
    //         li $t2,123
    //         sub $t2,$t0,$t2
    //         add $t2,$t0,$t2
    //         sw $t2,-4($sp)
    //         la $t3,string1
    //         sw $t3,-8($sp)
    //         lw $t4,-8($sp)
    //         sw $t4,-12($sp)
    //
    //         move $2, $0
    //         j $31
}
