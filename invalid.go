package main

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

// GenerateInvalidUTF8 genera un valor UTF-8 inválido según el tipo especificado
func GenerateInvalidUTF8(invalidType InvalidUTF8Type) []byte {
	rand.Seed(time.Now().UnixNano())

	switch invalidType {
	case IncompleteSequence:
		rand.Seed(time.Now().UnixNano())
		return []byte{0xC0 + byte(rand.Intn(0x20))}
	case ContinuationByteOnly:
		rand.Seed(time.Now().UnixNano())
		return []byte{0x80 + byte(rand.Intn(0x40))}
	case OverlongSequence:
		return []byte{0xC0, 0x81}
	case InvalidByteRange:
		rand.Seed(time.Now().UnixNano())
		return []byte{0xF5 + byte(rand.Intn(0x0B))}
	case SurrogateHalf:
		rand.Seed(time.Now().UnixNano())
		return []byte{0xED, 0xA0 + byte(rand.Intn(0x20))}
	case RandomInvalid:
		rand.Seed(time.Now().UnixNano())
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
		rand.Seed(time.Now().UnixNano())
		return []byte{0xC0}
	}
}

// GenerateInvalidUTF8String genera un string con valores UTF-8 inválidos
func GenerateInvalidUTF8String(invalidType InvalidUTF8Type) string {
	return string(GenerateInvalidUTF8(invalidType))
}

// GenerateMultipleInvalidUTF8 genera múltiples valores UTF-8 inválidos
func GenerateMultipleInvalidUTF8(count int, invalidType InvalidUTF8Type) [][]byte {
	result := make([][]byte, count)
	for i := 0; i < count; i++ {
		result[i] = GenerateInvalidUTF8(invalidType)
	}
	return result
}

// GenerateMixedInvalidUTF8 genera una secuencia que mezcla bytes válidos e inválidos
func GenerateMixedInvalidUTF8(validPrefix string, invalidType InvalidUTF8Type, validSuffix string) []byte {
	result := []byte(validPrefix)
	result = append(result, GenerateInvalidUTF8(invalidType)...)
	result = append(result, []byte(validSuffix)...)
	return result
}

// GetAllInvalidUTF8Types retorna ejemplos de todos los tipos de UTF-8 inválidos
// Útil para pruebas exhaustivas
func GetAllInvalidUTF8Types() map[string][]byte {
	types := map[string][]byte{
		"incomplete_sequence":    GenerateInvalidUTF8(IncompleteSequence),
		"continuation_byte_only": GenerateInvalidUTF8(ContinuationByteOnly),
		"overlong_sequence":      GenerateInvalidUTF8(OverlongSequence),
		"invalid_byte_range":     GenerateInvalidUTF8(InvalidByteRange),
		"surrogate_half":         GenerateInvalidUTF8(SurrogateHalf),
		"random_invalid":         GenerateInvalidUTF8(RandomInvalid),
	}
	return types
}

// IsValidUTF8 verifica si un []byte contiene UTF-8 válido
func IsValidUTF8(data []byte) bool {
	return utf8.Valid(data)
}

// ValidateAndGetInvalidUTF8 genera un UTF-8 inválido y verifica que realmente sea inválido
// Retorna el []byte inválido y un error si no se pudo generar uno inválido
func ValidateAndGetInvalidUTF8(invalidType InvalidUTF8Type) ([]byte, error) {
	invalid := GenerateInvalidUTF8(invalidType)
	if utf8.Valid(invalid) {
		// Si por alguna razón es válido, intenta generar otro
		for i := 0; i < 10; i++ {
			invalid = GenerateInvalidUTF8(RandomInvalid)
			if !utf8.Valid(invalid) {
				return invalid, nil
			}
		}
		return nil, &InvalidUTF8GenerationError{
			Message: "No se pudo generar un UTF-8 inválido después de múltiples intentos",
		}
	}
	return invalid, nil
}

// InvalidUTF8GenerationError es un error personalizado para fallos en la generación
type InvalidUTF8GenerationError struct {
	Message string
}

func (e *InvalidUTF8GenerationError) Error() string {
	return e.Message
}

// GetInvalidUTF8ForConfig retorna un string con UTF-8 inválido que puede ser usado en campos del config

func GetInvalidUTF8ForConfig(invalidType InvalidUTF8Type) string {
	invalid, err := ValidateAndGetInvalidUTF8(invalidType)
	if err != nil {
		// Si falla, retorna al menos una secuencia conocida inválida
		invalid = []byte{0xC0, 0x80}
	}
	return string(invalid)
}

// GetInvalidUTF8Base64 retorna el valor UTF-8 inválido codificado en Base64
// Útil para insertar en configs YAML que requieren valores codificados
func GetInvalidUTF8Base64(invalidType InvalidUTF8Type) string {
	invalid := GenerateInvalidUTF8(invalidType)
	// Para usar Base64, necesitarías importar encoding/base64
	// Por ahora, retornamos el string directamente
	return string(invalid)
}

// GetInvalidUTF8Hex retorna el valor UTF-8 inválido como string hexadecimal
func GetInvalidUTF8Hex(invalidType InvalidUTF8Type) string {
	invalid := GenerateInvalidUTF8(invalidType)
	return fmt.Sprintf("%X", invalid)
}

// FormatInvalidUTF8ForYAML retorna el valor formateado para usar en YAML
func FormatInvalidUTF8ForYAML(invalidType InvalidUTF8Type, fieldName string) string {
	invalid := GenerateInvalidUTF8(invalidType)
	return fmt.Sprintf(`%s: "%s"`, fieldName, string(invalid))
}

/*
Ejemplos de uso:

	// Generar un valor UTF-8 inválido simple
	invalidBytes := GenerateInvalidUTF8(IncompleteSequence)
	invalidString := string(invalidBytes)

	// Usar en un campo del config
	description := GetInvalidUTF8ForConfig(ContinuationByteOnly)
	// Luego puedes usar 'description' en campos del YAML como:
	//   Description: "{{ description }}"

	// Obtener todos los tipos de valores inválidos para pruebas exhaustivas
	allInvalidTypes := GetAllInvalidUTF8Types()
	for name, invalidBytes := range allInvalidTypes {
		fmt.Printf("Tipo: %s, Bytes: %X, Válido: %v\n", name, invalidBytes, IsValidUTF8(invalidBytes))
	}

	// Generar múltiples valores inválidos
	multipleInvalid := GenerateMultipleInvalidUTF8(5, RandomInvalid)

	// Mezclar bytes válidos e inválidos
	mixed := GenerateMixedInvalidUTF8("Texto válido: ", SurrogateHalf, " más texto")

	// Formatear para YAML
	yamlField := FormatInvalidUTF8ForYAML(InvalidByteRange, "Description")
	// Resultado: Description: "\xF5"

	// Obtener en formato hexadecimal para debugging
	hexValue := GetInvalidUTF8Hex(OverlongSequence)
	// Resultado: "C081"
*/
