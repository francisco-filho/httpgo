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

O exemplo precisa de uma API local na porta 8080 com `/health` retornando 200 e
`/inexistente` retornando 404. Os testes E2E fornecem servidores locais isolados;
não precisam dessa API externa.

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
**desenvolvimento**, não no binário entregue. A limitação da sandbox e o aceite
nativo pendente em macOS/Windows estão registrados em `docs/validation.md`;
a história não está declarada concluída.
