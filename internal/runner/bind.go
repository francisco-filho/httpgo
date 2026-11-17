package runner

import (
	"capimgrosso.com/httpgo/internal/query"
	"capimgrosso.com/httpgo/internal/template"
)

// bindQuery prepares operands only, never an expression. Resolution and secret
// protection precede decoding/selection, even when the response is empty.
func bindQuery(session template.Session, path query.Path) (query.Path, bool, error) {
	secret := false
	bound, err := path.Bind(func(source string, complete bool) (any, error) {
		refs, err := template.Parse(source) // Already validated; inspect provenance.
		if err != nil {
			return nil, err
		}
		operandSecret := false
		for _, ref := range refs {
			operandSecret = operandSecret || session.IsSecret(ref.Name)
		}
		secret = secret || operandSecret
		if complete {
			raw, err := session.RenderJSON([]byte(source))
			if err != nil {
				return nil, err
			}
			if operandSecret {
				session.ProtectValue(string(raw))
			}
			return query.Decode(raw)
		}
		text, err := session.Render(source)
		if err != nil {
			return nil, err
		}
		if operandSecret {
			session.ProtectValue(text.Value)
		}
		return text.Value, nil
	})
	return bound, secret, err
}
