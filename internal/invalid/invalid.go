package invalid

import (
	"fmt"
	"math/rand"
	"time"
	"unicode/utf8"
)

// InvalidUTF8Type representa diferentes tipos de valores UTF-8 inválidos
type InvalidUTF8Type int

const (
	IncompleteSequence InvalidUTF8Type = iota
	ContinuationByteOnly
	OverlongSequence
	InvalidByteRange
	SurrogateHalf
	RandomInvalid
)

func GenerateInvalidUTF8(invalidType InvalidUTF8Type) []byte {
	rand.Seed(time.Now().UnixNano())

	switch invalidType {
	case IncompleteSequence:
		return []byte{0xC0 + byte(rand.Intn(0x20))}
	case ContinuationByteOnly:
		return []byte{0x80 + byte(rand.Intn(0x40))}
	case OverlongSequence:
		return []byte{0xC0, 0x81}
	case InvalidByteRange:
		return []byte{0xF5 + byte(rand.Intn(0x0B))}
	case SurrogateHalf:
		return []byte{0xED, 0xA0 + byte(rand.Intn(0x20))}
	case RandomInvalid:
		length := rand.Intn(4) + 1
		result := make([]byte, length)
		for i := 0; i < length; i++ {
			result[i] = byte(rand.Intn(256))
		}
		for utf8.Valid(result) {
			result[0] = byte(rand.Intn(256))
		}
		return result
	default:
		return []byte{0xC0}
	}
}

func GetInvalidUTF8ForConfig(invalidType InvalidUTF8Type) string {
	invalid := GenerateInvalidUTF8(invalidType)
	if utf8.Valid(invalid) {
		invalid = []byte{0xC0, 0x80}
	}
	return string(invalid)
}

func GetInvalidUTF8String(invalidType InvalidUTF8Type) string {
	return string(GenerateInvalidUTF8(invalidType))
}

func IsValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

func GetInvalidUTF8ByTypeName(typeName string) string {
	var invalidType InvalidUTF8Type
	switch typeName {
	case "incomplete":
		invalidType = IncompleteSequence
	case "continuation":
		invalidType = ContinuationByteOnly
	case "overlong":
		invalidType = OverlongSequence
	case "invalid_range":
		invalidType = InvalidByteRange
	case "surrogate":
		invalidType = SurrogateHalf
	case "random":
		invalidType = RandomInvalid
	default:
		invalidType = RandomInvalid
	}
	return GetInvalidUTF8ForConfig(invalidType)
}

func GetInvalidUTF8Hex(invalidType InvalidUTF8Type) string {
	invalid := GenerateInvalidUTF8(invalidType)
	return fmt.Sprintf("%X", invalid)
}

// GenerateValidUTF8 genera un valor UTF-8 válido aleatorio
// Útil para comparar con valores inválidos en pruebas
func GenerateValidUTF8() string {
	rand.Seed(time.Now().UnixNano())

	// Genera caracteres UTF-8 válidos aleatorios
	validChars := []rune{
		'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z',
		'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z',
		'0', '1', '2', '3', '4', '5', '6', '7', '8', '9',
		' ', '!', '@', '#', '$', '%', '^', '&', '*', '(', ')', '-', '_', '=', '+',
		'á', 'é', 'í', 'ó', 'ú', 'ñ', 'Ñ', 'ü', 'Ü',
		'€', '£', '¥', '©', '®', '™',
	}

	length := rand.Intn(20) + 5 // Entre 5 y 25 caracteres
	result := make([]rune, length)
	for i := 0; i < length; i++ {
		result[i] = validChars[rand.Intn(len(validChars))]
	}

	return string(result)
}
