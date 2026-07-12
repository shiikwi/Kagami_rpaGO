package pickle

import (
	"encoding/binary"
	"errors"
	"fmt"
)

type Tuple []any
type Dict map[string]any
type List []any

type mark struct{}

func Parse(data []byte) (any, error) {
	var (
		stack []any
		memo  []any
		pos   int
	)

	push := func(v any) {
		stack = append(stack, v)
	}

	pop := func() (any, error) {
		if len(stack) == 0 {
			return nil, errors.New("pickle stack underflow")
		}
		v := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		return v, nil
	}

	findMark := func() int {
		for i := len(stack) - 1; i >= 0; i-- {
			if _, ok := stack[i].(mark); ok {
				return i
			}
		}
		return -1
	}

	for pos < len(data) {
		op := data[pos]
		pos++

		switch op {
		case 0x80: // PROTO
			if pos >= len(data) {
				return nil, errors.New("truncated PROTO")
			}
			pos++

		case 0x95: // FRAME
			if pos+8 > len(data) {
				return nil, errors.New("truncated FRAME")
			}
			pos += 8

		case '}': // EMPTY_DICT
			push(Dict{})

		case ']': // EMPTY_LIST
			list := List{}
			push(&list)

		case '(': // MARK
			push(mark{})

		case 0x94: // MEMOIZE
			if len(stack) == 0 {
				return nil, errors.New("MEMOIZE on empty stack")
			}
			memo = append(memo, stack[len(stack)-1])

		case 'h': // BINGET
			if pos >= len(data) {
				return nil, errors.New("truncated BINGET")
			}
			index := int(data[pos])
			pos++
			if index >= len(memo) {
				return nil, fmt.Errorf("memo index out of range: %d", index)
			}
			push(memo[index])

		case 'j': // LONG_BINGET
			if pos+4 > len(data) {
				return nil, errors.New("truncated LONG_BINGET")
			}
			index := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
			pos += 4
			if index >= len(memo) {
				return nil, fmt.Errorf("memo index out of range: %d", index)
			}
			push(memo[index])

		case 'J': // BININT
			if pos+4 > len(data) {
				return nil, errors.New("truncated BININT")
			}
			v := int64(int32(binary.LittleEndian.Uint32(data[pos : pos+4])))
			pos += 4
			push(v)

		case 'K': // BININT1
			if pos >= len(data) {
				return nil, errors.New("truncated BININT1")
			}
			push(int64(data[pos]))
			pos++

		case 'M': // BININT2
			if pos+2 > len(data) {
				return nil, errors.New("truncated BININT2")
			}
			push(int64(binary.LittleEndian.Uint16(data[pos : pos+2])))
			pos += 2

		case 0x8c: // SHORT_BINUNICODE
			s, next, err := readShortString(data, pos)
			if err != nil {
				return nil, err
			}
			pos = next
			push(s)

		case 'X': // BINUNICODE
			if pos+4 > len(data) {
				return nil, errors.New("truncated BINUNICODE")
			}
			n := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
			pos += 4
			if pos+n > len(data) {
				return nil, errors.New("truncated BINUNICODE payload")
			}
			push(string(data[pos : pos+n]))
			pos += n

		case 'U': // SHORT_BINSTRING
			s, next, err := readShortString(data, pos)
			if err != nil {
				return nil, err
			}
			pos = next
			push(s)

		case 'C': // SHORT_BINBYTES
			b, next, err := readShortBytes(data, pos)
			if err != nil {
				return nil, err
			}
			pos = next
			push(b)

		case 'B': // BINBYTES
			if pos+4 > len(data) {
				return nil, errors.New("truncated BINBYTES")
			}
			n := int(binary.LittleEndian.Uint32(data[pos : pos+4]))
			pos += 4
			if pos+n > len(data) {
				return nil, errors.New("truncated BINBYTES payload")
			}
			b := append([]byte(nil), data[pos:pos+n]...)
			push(b)
			pos += n

		case 'N': // NONE
			push(nil)

		case 0x86: // TUPLE2
			b, err := pop()
			if err != nil {
				return nil, err
			}
			a, err := pop()
			if err != nil {
				return nil, err
			}
			push(Tuple{a, b})

		case 0x87: // TUPLE3
			c, err := pop()
			if err != nil {
				return nil, err
			}
			b, err := pop()
			if err != nil {
				return nil, err
			}
			a, err := pop()
			if err != nil {
				return nil, err
			}
			push(Tuple{a, b, c})

		case 'a': // APPEND
			item, err := pop()
			if err != nil {
				return nil, err
			}
			if len(stack) == 0 {
				return nil, errors.New("APPEND without target list")
			}
			list, ok := stack[len(stack)-1].(*List)
			if !ok {
				return nil, fmt.Errorf("APPEND target is %T", stack[len(stack)-1])
			}
			*list = append(*list, item)

		case 'u': // SETITEMS
			markIndex := findMark()
			if markIndex < 1 {
				return nil, errors.New("SETITEMS without dict/mark")
			}
			items := append([]any(nil), stack[markIndex+1:]...)
			stack = stack[:markIndex]

			dict, ok := stack[len(stack)-1].(Dict)
			if !ok {
				return nil, fmt.Errorf("SETITEMS target is %T", stack[len(stack)-1])
			}
			if len(items)%2 != 0 {
				return nil, errors.New("SETITEMS with odd item count")
			}
			for i := 0; i < len(items); i += 2 {
				key, ok := items[i].(string)
				if !ok {
					return nil, fmt.Errorf("SETITEMS key is %T", items[i])
				}
				dict[key] = items[i+1]
			}

		case '.': // STOP
			if len(stack) != 1 {
				return nil, fmt.Errorf("unexpected stack size at STOP: %d", len(stack))
			}
			return stack[0], nil

		default:
			return nil, fmt.Errorf("unsupported pickle opcode 0x%02X at offset %d", op, pos-1)
		}
	}

	return nil, errors.New("pickle ended without STOP")
}

func readShortString(data []byte, pos int) (string, int, error) {
	if pos >= len(data) {
		return "", pos, errors.New("truncated short string")
	}
	n := int(data[pos])
	pos++
	if pos+n > len(data) {
		return "", pos, errors.New("truncated short string payload")
	}
	return string(data[pos : pos+n]), pos + n, nil
}

func readShortBytes(data []byte, pos int) ([]byte, int, error) {
	if pos >= len(data) {
		return nil, pos, errors.New("truncated short bytes")
	}
	n := int(data[pos])
	pos++
	if pos+n > len(data) {
		return nil, pos, errors.New("truncated short bytes payload")
	}
	return append([]byte(nil), data[pos:pos+n]...), pos + n, nil
}
