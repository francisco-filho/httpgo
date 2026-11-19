package runner

import (
	"encoding/json"
	"fmt"
	"regexp"

	"capimgrosso.com/httpgo/internal/assertion"
	"capimgrosso.com/httpgo/internal/parser"
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// RegexFailure deliberately retains neither response text nor selected group.
type RegexFailure struct {
	Position          parser.Position
	Pattern, Expected string
	Operator          assertion.HeaderOperator
	Present           bool
}

// RegexError is categorical and never wraps raw regex or body contents.
type RegexError struct {
	Position parser.Position
	Operator assertion.HeaderOperator
	Detail   string
}

func (e *RegexError) Error() string {
	return fmt.Sprintf("erro de avaliação regex %s: %s", e.Operator, e.Detail)
}

// JSONFailure contains only sanitized presentation values, not the document.
// Binary operands are encoded JSON; type/empty predicates retain categories only.
type JSONFailure struct {
	Position         parser.Position
	Query            string
	Expected, Actual string
	Present          bool
	Operator         assertion.JSONOperator
	Count            bool
	Detail           string
}

// JSONError is a controlled evaluation error, never a raw decoding cause.
type JSONError struct {
	Position      parser.Position
	Query, Detail string
}

func (e *JSONError) Error() string {
	return fmt.Sprintf("erro de avaliação JSON: consulta %q: %s", e.Query, e.Detail)
}

// HeaderError is a controlled expected-value evaluation error, not a mismatch.
type HeaderError struct {
	Position     parser.Position
	Name, Detail string
}

func (e *HeaderError) Error() string {
	return fmt.Sprintf("erro de avaliação de header %q: %s", e.Name, e.Detail)
}

// prepareTextExpected resolves once and protects complete secret derivatives before
// compilation, comparison or presentation. It never reinterprets inserted text.
func prepareTextExpected(session template.Session, plan *parser.ExpectedTemplate, sensitive bool) (string, bool, error) {
	secret := sensitive
	for _, ref := range plan.References {
		secret = secret || session.IsSecret(ref.Name)
	}
	text, err := session.Render(plan.Source)
	if err != nil {
		return "", secret, err
	}
	if secret {
		session.ProtectValue(text.Value)
	}
	return text.Value, secret, nil
}

func compileExpected(pattern string) (*regexp.Regexp, error) {
	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("padrão matches inválido ou não suportado")
	}
	return compiled, nil
}

// BodyFailure contains presentation metadata only, never response text.
type BodyFailure struct {
	Position parser.Position
	Operator assertion.BodyOperator
	Expected string
}

// BodyError has a controlled categorical cause, never raw decoding input.
type BodyError struct {
	Position parser.Position
	Operator assertion.BodyOperator
	Detail   string
}

func (e *BodyError) Error() string {
	return fmt.Sprintf("erro de avaliação de body %s: %s", e.Operator, e.Detail)
}

// typeFailure never encodes an operand or invents an expected JSON literal.
// Even categorical metadata is concealed when derived from protected content.
func typeFailure(session template.Session, declaration parser.JSONAssertion, pos parser.Position, actual any, present bool, evaluation assertion.JSONEvaluation, conceal bool) *JSONFailure {
	failure := &JSONFailure{Position: pos, Query: session.Sanitize(declaration.Path.RedactLiterals(declaration.Query)), Present: present, Operator: declaration.Operator}
	protectedLiteral := declaration.Path.HasProtectedLiteral(func(value any) bool { return protectedJSON(session, value) })
	if conceal || protectedLiteral || session.Sanitize(declaration.Query) != declaration.Query || protectedJSON(session, actual) {
		failure.Actual = "[oculto]"
		return failure
	}
	failure.Actual = "ausente"
	if present {
		failure.Actual = "tipo " + assertion.JSONType(actual)
	}
	failure.Detail = evaluation.Detail
	if session.Sanitize(failure.Actual) != failure.Actual || session.Sanitize(failure.Detail) != failure.Detail {
		failure.Actual, failure.Detail = "[oculto]", ""
	}
	return failure
}

func checkAssertions(session template.Session, entry parser.Entry, response Response, current *EntryResult) error {
	var document any
	var text string
	decoded, textDecoded := false, false
	conceal := pendingSecret(session, entry)
	for _, item := range entry.Assertions {
		switch {
		case item.Regex != nil && item.Body == nil && item.Header == nil && item.JSON == nil:
			declaration := *item.Regex
			pos := declaration.Position
			pos.File = session.Sanitize(pos.File)
			fail := func(detail string) error {
				current.Outcome = RegexFailed
				current.RegexError = &RegexError{Position: pos, Operator: declaration.Operator, Detail: session.Sanitize(detail)}
				return current.RegexError
			}
			if plan := declaration.ExpectedTemplate; plan != nil {
				value, secret, err := prepareTextExpected(session, plan, false)
				if err == nil && declaration.Operator == assertion.HeaderMatches {
					declaration.Compiled, err = compileExpected(value)
				}
				if err != nil {
					return fail("esperado textual inválido: " + err.Error())
				}
				declaration.Expected = value
				conceal = conceal || secret
			}
			pos = declaration.QueryPosition
			pos.File = session.Sanitize(pos.File)
			if !textDecoded {
				var err error
				text, err = query.BodyText(response.Body, response.Headers)
				if err != nil {
					return fail(err.Error())
				}
				textDecoded = true
			}
			actual, present, err := query.SelectRegex(declaration.Extraction, text)
			if err != nil {
				return fail(err.Error())
			}
			matched := false
			if declaration.Operator == assertion.HeaderMatches {
				matched = assertion.MatchString(declaration.Compiled, actual, present)
			} else {
				matched = assertion.Header(declaration.Operator, declaration.Expected, actual, present)
			}
			if !matched {
				pattern, expected := "[oculto]", "[oculto]"
				if !conceal {
					pattern, expected = session.Sanitize(declaration.Pattern), session.Sanitize(declaration.Expected)
				}
				pos = declaration.Position
				pos.File = session.Sanitize(pos.File)
				current.RegexFailure = &RegexFailure{Position: pos, Pattern: pattern, Expected: expected, Operator: declaration.Operator, Present: present}
				current.Outcome = RegexMismatch
				return nil
			}
		case item.Body != nil && item.Header == nil && item.JSON == nil && item.Regex == nil:
			declaration := *item.Body
			pos := declaration.Position
			pos.File = session.Sanitize(pos.File)
			if plan := declaration.ExpectedTemplate; plan != nil {
				value, secret, err := prepareTextExpected(session, plan, false)
				if err != nil {
					current.Outcome = BodyFailed
					current.BodyError = &BodyError{Position: pos, Operator: declaration.Operator, Detail: "não foi possível resolver o esperado textual: " + err.Error()}
					return current.BodyError
				}
				declaration.Expected = value
				conceal = conceal || secret
			}
			if !textDecoded {
				var err error
				text, err = query.BodyText(response.Body, response.Headers)
				if err != nil {
					current.Outcome = BodyFailed
					current.BodyError = &BodyError{Position: pos, Operator: declaration.Operator, Detail: err.Error()}
					return current.BodyError
				}
				textDecoded = true
			}
			if !assertion.Body(declaration.Operator, declaration.Expected, text) {
				expected := "[oculto]"
				if !conceal {
					expected = session.Sanitize(declaration.Expected)
				}
				current.BodyFailure = &BodyFailure{Position: pos, Operator: declaration.Operator, Expected: expected}
				current.Outcome = BodyMismatch
				return nil
			}
		case item.Header != nil && item.JSON == nil && item.Body == nil && item.Regex == nil:
			declaration := *item.Header
			if plan := declaration.ExpectedTemplate; plan != nil {
				value, secret, err := prepareTextExpected(session, plan, sensitiveHeader(declaration.Name))
				if err == nil && declaration.Operator == assertion.HeaderMatches {
					declaration.Compiled, err = compileExpected(value)
				}
				if err != nil {
					pos := declaration.Position
					pos.File = session.Sanitize(pos.File)
					current.Outcome = HeaderFailed
					current.HeaderError = &HeaderError{Position: pos, Name: session.Sanitize(declaration.Name), Detail: "esperado textual inválido: " + err.Error()}
					return current.HeaderError
				}
				declaration.Expected = value
				conceal = conceal || secret
			}
			current.HeaderFailure = checkHeader(session, declaration, response.Headers, conceal)
			if current.HeaderFailure != nil {
				current.Outcome = HeaderMismatch
				return nil
			}
		case item.JSON != nil && item.Header == nil && item.Body == nil && item.Regex == nil:
			resolved := *item.JSON // Evaluation must never mutate the reusable scenario.
			declaration := &resolved
			pos := declaration.Position
			pos.File = session.Sanitize(pos.File)
			fail := func(detail string) *JSONError {
				current.Outcome = JSONFailed
				return &JSONError{Position: pos, Query: session.Sanitize(declaration.Path.RedactLiterals(declaration.Query)), Detail: detail}
			}
			if plan := declaration.ExpectedTemplate; plan != nil && declaration.Operator == assertion.JSONMatches {
				value, secret, err := prepareTextExpected(session, plan, false)
				if err != nil {
					return fail("não foi possível resolver o padrão: " + err.Error())
				}
				declaration.Expected, declaration.Pattern = value, value
				declaration.Compiled, err = compileExpected(value)
				if err != nil {
					return fail(err.Error())
				}
				conceal = conceal || secret
			} else if plan != nil {
				secret := false
				for _, ref := range plan.JSON.References {
					secret = secret || session.IsSecret(ref.Name)
				}
				rendered, err := session.RenderJSON([]byte(plan.Source))
				if err != nil {
					return fail("não foi possível resolver o esperado JSON: " + err.Error())
				}
				if secret {
					session.ProtectValue(string(rendered))
					conceal = true
				}
				declaration.Expected, err = query.Decode(rendered)
				if err != nil {
					return fail("esperado JSON renderizado inválido")
				}
				if err := assertion.ValidateJSONExpected(declaration.Operator, declaration.Count, declaration.Expected); err != nil {
					return fail(err.Error())
				}
			}
			bound, secret, err := bindQuery(session, declaration.Path)
			if err != nil {
				pos = declaration.QueryPosition
				pos.File = session.Sanitize(pos.File)
				return fail("não foi possível resolver operando JSONPath: " + err.Error())
			}
			declaration.Path = bound
			conceal = conceal || secret
			if !decoded {
				var err error
				document, err = query.Decode(response.Body)
				if err != nil {
					return fail("resposta não contém um único valor JSON válido")
				}
				decoded = true
			}
			actual, present, err := declaration.Path.SelectValue(document)
			if err != nil {
				return fail(err.Error())
			}
			var evaluation assertion.JSONEvaluation
			if declaration.Operator == assertion.JSONMatches {
				_, isString := actual.(string)
				evaluation = assertion.JSONEvaluation{Actual: actual, Matched: assertion.MatchString(declaration.Compiled, actual, present)}
				if present && !isString {
					evaluation.Detail = "matches exige string; recebido " + assertion.JSONType(actual)
				}
			} else {
				evaluation, err = assertion.EvaluateJSON(declaration.Operator, declaration.Count, declaration.Expected, actual, present)
			}
			if err != nil {
				return fail(err.Error())
			}
			if evaluation.Matched {
				continue
			}
			if declaration.Operator.TypePredicate() {
				current.JSONFailure = typeFailure(session, *declaration, pos, actual, present, evaluation, conceal)
				current.Outcome = JSONMismatch
				return nil
			}
			if declaration.Path.Expansive() || declaration.Operator == assertion.JSONNotEqualOperator {
				current.JSONFailure = projectionFailure(session, *declaration, pos, actual, present, evaluation, conceal)
				current.Outcome = JSONMismatch
				return nil
			}
			expectedText, actualText := "[oculto]", "[oculto]"
			if !conceal {
				expectedJSON, err := json.Marshal(declaration.Expected)
				if err != nil {
					return fail("não foi possível representar o esperado JSON")
				}
				actualJSON, err := json.Marshal(evaluation.Actual)
				if err != nil {
					return fail("não foi possível representar o valor JSON selecionado")
				}
				expectedText, actualText = string(expectedJSON), string(actualJSON)
				if declaration.Operator == assertion.JSONNotExists {
					expectedText, actualText = "ausente", "presente"
				} else if declaration.Operator == assertion.JSONContains || declaration.Operator == assertion.JSONNotContains {
					// Evaluation deliberately discarded the selected collection. Report
					// only its type, never marshal the response value for membership.
					actualText = "tipo " + assertion.JSONType(actual)
				} else if declaration.Operator == assertion.JSONExists {
					expectedText, actualText = "presente", "presente"
				} else if declaration.Operator == assertion.JSONMatches {
					patternJSON, _ := json.Marshal(declaration.Pattern)
					actualJSON, _ := json.Marshal(evaluation.Actual)
					expectedText = string(patternJSON)
					if !present {
						actualText = "ausente"
					} else if evaluation.Detail != "" {
						actualText = "tipo " + assertion.JSONType(actual)
					} else {
						actualText = string(actualJSON)
					}
				} else if evaluation.Detail != "" {
					actualText = "tipo " + assertion.JSONType(actual)
				}
				expectedText, actualText = session.Sanitize(expectedText), session.Sanitize(actualText)
			}
			current.JSONFailure = &JSONFailure{Position: pos, Query: session.Sanitize(declaration.Query), Expected: expectedText, Actual: actualText, Present: present, Operator: declaration.Operator, Count: declaration.Count, Detail: session.Sanitize(evaluation.Detail)}
			current.Outcome = JSONMismatch
			return nil
		default:
			current.Outcome = JSONFailed
			return &JSONError{Position: entry.Position, Detail: "variante de assertion não suportada"}
		}
	}
	return nil
}
