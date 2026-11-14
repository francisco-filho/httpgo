# httpgo

CLI independente em Go para executar um **subconjunto documentado** de arquivos
`.hurl`. Implementação própria, sem reutilizar código-fonte do Hurl.

## Executar

Com Go na versão de `go.mod`:

```sh
CGO_ENABLED=0 make build
bin/httpgo --help
bin/httpgo test testdata/scenarios/scenario_get_status.hurl
```

Para usar o servidor dummy do projeto, execute em outro terminal:

```sh
make serve
# Alias equivalente: make dummy
```

Ele escuta somente em `http://127.0.0.1:8080` (também acessível via localhost),
em primeiro plano. Ctrl+C encerra o servidor. A porta precisa estar livre.

| GET | Status |
|---|---|
| `/health` | 200 |
| `/inexistente` | 404 |
| `/error` | 500 |
| Outros caminhos | 404 |

O cenário `scenario_get_status.hurl` executa `/health`, `/inexistente` e repete
`/health`, validando três respostas em sequência. Também é possível executar:

```sh
bin/httpgo test testdata/scenarios/scenario_unvalidated_status.hurl
```

Esse cenário recebe 500 e informa explicitamente que não validou status.
O dummy em `tools/dummy/main.go` é um utilitário **exclusivamente manual** de
desenvolvimento, sem dependências novas. Não integra o binário httpgo nem os
pacotes do build, lint ou testes automáticos. Os testes continuam usando seus
próprios servidores isolados em portas dinâmicas: **não precisam de `make serve`**.
Veja [servidor dummy](docs/dev-server.md) para os detalhes.

```hurl
GET http://localhost:8080/health
HTTP 200

GET http://localhost:8080/inexistente
HTTP 404
```

O arquivo inteiro é validado antes da rede. GETs são sequenciais e a primeira
falha interrompe a execução. Sem `HTTP`, qualquer status recebido é aceito,
com indicação explícita de **sem validação de status**.

- `0`: sucesso; `1`: status divergente; `2`: demais erros.
- Timeout total de 10 segundos por GET, incluindo redirects e leitura do body.
- TLS verificado; até 10 redirects, apenas na mesma origem; nenhum retry.
- Resultados e resumo em stdout; diagnósticos em stderr. Sempre texto sem cores.
- Credenciais, query inteira e fragmento da URL não são exibidos.
  **Não coloque segredos no path: ele permanece visível.**

## Documentação e desenvolvimento

- [Subconjunto e contratos da CLI](docs/subset.md)
- [Arquitetura e ciclo de vida](docs/architecture.md)
- [Servidor dummy para desenvolvimento](docs/dev-server.md)
- [Compatibilidade diferencial com Hurl 8.0.0](docs/compatibility.md)
- [Comandos, evidências e bloqueios de validação](docs/validation.md)
- [Regras do repositório](AGENTS.md)

```sh
make fmt lint test
# Explicitamente, além do lint:
go vet ./...
make build
make compatibility
make portable
```

Os testes com `-race` exigem CGO e compilador/headers C no ambiente de
**desenvolvimento**, não no binário entregue. O gate local com race já passou;
o aceite nativo pendente em macOS/Windows está registrado em
`docs/validation.md`. A história não está declarada concluída.
