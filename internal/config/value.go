package config

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

func jsonValue(file string, node *yaml.Node) (any, error) {
	if node.Kind == yaml.ScalarNode && strings.Contains(node.Value, "${") {
		return nil, failure(file, node, "referências ${NOME} dentro de listas não são suportadas", nil)
	}
	switch node.Kind {
	case yaml.SequenceNode:
		values := make([]any, 0, len(node.Content))
		for _, child := range node.Content {
			value, err := jsonValue(file, child)
			if err != nil {
				return nil, err
			}
			values = append(values, value)
		}
		return values, nil
	case yaml.MappingNode:
		values := make(map[string]any)
		for i := 0; i < len(node.Content); i += 2 {
			if err := rejectReferences(file, node.Content[i]); err != nil {
				return nil, err
			}
			value, err := jsonValue(file, node.Content[i+1])
			if err != nil {
				return nil, err
			}
			values[node.Content[i].Value] = value
		}
		return values, nil
	case yaml.ScalarNode:
		switch node.Tag {
		case "!!str":
			// The YAML resolver uses machine floats to infer tags and can label
			// large/very small plain numbers as strings. Resolve their spelling
			// exactly; quoted numeric strings must remain strings.
			if node.Style == 0 {
				if number, err := numberValue(node); err == nil {
					return json.Number(number), nil
				}
			}
			return node.Value, nil
		case "!!null":
			return nil, nil
		case "!!bool":
			return strings.EqualFold(node.Value, "true"), nil
		case "!!int", "!!float":
			number, err := numberValue(node)
			if err == nil {
				return json.Number(number), nil
			}
		default:
			return nil, failure(file, node, "tipo de valor não suportado; use valores JSON compatíveis", nil)
		}
	default:
		return nil, failure(file, node, "estrutura de valor não suportada", nil)
	}
	return nil, failure(file, node, "número deve ser finito e representável em JSON", nil)
}

// Convert spelling, never magnitude through float64. JSON exponents stay compact.
func numberValue(node *yaml.Node) (string, error) {
	if node.Kind != yaml.ScalarNode || (node.Tag != "!!int" && node.Tag != "!!float" && (node.Tag != "!!str" || node.Style != 0)) {
		return "", fmt.Errorf("tipo numérico inválido")
	}
	if node.Value == "" || !strings.ContainsRune("+-0123456789.", rune(node.Value[0])) {
		return "", fmt.Errorf("número inválido")
	}
	source := strings.ReplaceAll(node.Value, "_", "")
	if source == "" || !strings.ContainsRune("+-0123456789.", rune(source[0])) {
		return "", fmt.Errorf("número inválido")
	}
	if value, ok := new(big.Int).SetString(source, 0); ok {
		return value.String(), nil
	}
	if node.Tag == "!!int" {
		return "", fmt.Errorf("inteiro inválido")
	}
	source = strings.TrimPrefix(source, "+")
	mantissa, exponent, hasExponent := strings.Cut(strings.ToLower(source), "e")
	if !strings.ContainsAny(mantissa, "0123456789") {
		return "", fmt.Errorf("número inválido")
	}
	sign := ""
	if strings.HasPrefix(mantissa, "-") {
		sign = "-"
		mantissa = mantissa[1:]
	}
	integer, fraction, hasPoint := strings.Cut(mantissa, ".")
	integer = strings.TrimLeft(integer, "0")
	if integer == "" {
		integer = "0"
	}
	source = sign + integer
	if hasPoint && fraction != "" {
		source += "." + fraction
	}
	if hasExponent {
		source += "e" + exponent
	}
	if !json.Valid([]byte(source)) || strings.ContainsAny(source, "\"[]{}") {
		return "", fmt.Errorf("número inválido")
	}
	return source, nil
}

func timeoutValue(node *yaml.Node) (time.Duration, error) {
	number, err := numberValue(node)
	if err != nil {
		return 0, err
	}
	mantissa, exponent, _ := strings.Cut(number, "e")
	integer, fraction, _ := strings.Cut(mantissa, ".")
	digits := strings.TrimLeft(integer+fraction, "0")
	if digits == "" || strings.HasPrefix(digits, "-") {
		return 0, fmt.Errorf("timeout fora da faixa")
	}
	power := new(big.Int)
	if exponent != "" {
		if _, ok := power.SetString(exponent, 10); !ok {
			return 0, fmt.Errorf("expoente inválido")
		}
	}
	power.Add(power, big.NewInt(int64(3-len(fraction))))
	// Remove trailing zeros before checking integral milliseconds.
	trimmed := strings.TrimRight(digits, "0")
	power.Add(power, big.NewInt(int64(len(digits)-len(trimmed))))
	digits = trimmed
	if power.Sign() < 0 || power.Cmp(big.NewInt(8)) > 0 {
		return 0, fmt.Errorf("timeout não representável")
	}
	zeros := int(power.Int64())
	if len(digits)+zeros > 8 {
		return 0, fmt.Errorf("timeout fora da faixa")
	}
	milliseconds, err := strconv.ParseInt(digits+strings.Repeat("0", zeros), 10, 64)
	if err != nil || milliseconds < 1 || milliseconds > 86400000 {
		return 0, fmt.Errorf("timeout fora da faixa")
	}
	return time.Duration(milliseconds) * time.Millisecond, nil
}
