# Validação — história #4 / US-01

## Estado

**Implementação local disponível; entrega NÃO concluída e sem handoff final.**
O gate com race está bloqueado pela sandbox, e runtime macOS/Windows ainda não
foi executado. Não há skip silencioso, retirada de checks ou redução de cobertura
para declarar sucesso. Não foi executada análise SonarQube (responsabilidade QA).

## Ambiente e ferramentas

- Host de execução: Linux amd64, sandbox.
- Módulo existente: `capimgrosso.com/httpgo`; Go `1.26.10` conforme `go.mod`.
- Go observado: `go1.26.10-X:nodwarf5 linux/amd64`.
- Ferramentas de desenvolvimento instaladas com autorização, sem alterar go.mod:
  `goimports` de `golang.org/x/tools v0.52.0` e `golangci-lint v2.15.0`, compilados
  com o Go existente. Acrescente `$(go env GOPATH)/bin` ao PATH para usá-las.
- Referência diferencial: Hurl `8.0.0`, libcurl `8.18.0` já disponível na sandbox.
- GCC presente mas sem `cc1`; Clang presente, porém sem headers libc/pthread.
  Não foram instalados pré-requisitos C, conforme orientação do responsável.

Em ambiente novo com ferramentas C disponíveis, os comandos de instalação das
ferramentas Go de desenvolvimento são:

```sh
go install golang.org/x/tools/cmd/goimports@v0.52.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

A instalação dessas ferramentas não altera dependências do produto.

## Evidências executadas nesta sandbox

Com `CGO_ENABLED=0` e as ferramentas Go no PATH:

| Comando/cenário | Resultado |
|---|---|
| `make fmt lint` | passou; 0 issues, inclusive testes com tag compatibility no lint |
| `go test ./internal/parser` | passou |
| `go test ./internal/assertion ./internal/runner` | passou |
| `go test ./internal/transport` | passou, HTTP/TLS locais reais |
| `go test ./internal/cli` | passou, args e streams injetados, sem binário |
| `go test ./tests/e2e` | passou, binário real compilado uma vez por suíte |
| `go test ./... -cover -count=1` | passou, incluindo todos os E2E padrão |
| `go vet ./...` | passou explicitamente |
| `go build ./...` e `make build` | passaram |
| `make compatibility` | passou, 9 pares com Hurl 8.0.0; detalhes em compatibility.md |
| `make portable` | passou para linux/darwin/windows, GOARCH=amd64, CGO=0 |
| `gofmt -l` nos arquivos Go alterados | sem arquivos pendentes |
| `git diff --check` | passou |

Cobertura local observada: assertion 100%, runner 100%, parser 97,5%, CLI 93,0%,
transport 90,9%. `main` não é instrumentado pelos testes de pacote; é exercitado
pelo binário nos E2E, inclusive metadados via ldflags. Cobertura é observação, não
um limite reduzido ou substituto do gate com race.

Entre os E2E efetivamente executados no Linux:

- demonstração com dois GETs em ordem e caso 503 interrompendo antes do segundo;
- 500 sem expectativa, resultado explícito sem validação;
- parsing integral com erro tardio, URL inválida, construção não suportada,
  entrada vazia/ilegível, quantidade incorreta e flags desconhecidas;
- redirects, excesso de saltos, mudança de origem e Location inválido, sem envio
  aos destinos proibidos ou GET seguinte;
- TLS inválido, conexão recusada, resposta incompleta e conexão encerrada sem retry;
- timeout público real de **10 segundos**, incluindo redirect e body;
- **Ctrl+C real** após o servidor confirmar a requisição em andamento, exit 2 e
  nenhuma requisição seguinte;
- stdout/stderr informativos sem userinfo, query ou fragmento; ajuda e versão;
- pipe, arquivo e **PTY Linux real**, sem cores e com NO_COLOR vazio/definido.

Os testes internos de transporte observam o mesmo deadline nos redirects reais,
com timeout menor apenas nesses testes, e não usam sleep para sincronização.
O teste de retry inclui warm-up e falha de conexão posterior.

## Bloqueio: gate obrigatório com race

`make test` continua executando `go test ./... -race -cover`, sem fallback.

- Com CGO desabilitado: `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`.
- Com CGO habilitado, GCC falha por ausência de `cc1`; Clang falha por ausência
  de `stdio.h`, `stdlib.h`, `pthread.h` e demais headers C.

Assim, `make fmt lint test` **não passou integralmente**. Os resultados sem CGO
acima são evidência parcial útil e não equivalem a sucesso desse gate. Nenhuma
task foi marcada `done` na fila em consequência dessa validação parcial.

O responsável informou que a preparação C ficará para a próxima história. Este
registro preserva a pendência; **não cria outra task nem transfere ou remove o gate
exigido pela história #4**. Quando um ambiente com C estiver disponível, executar:

```sh
make fmt lint test
go vet ./...
make build compatibility portable
```

Não é necessário CGO para o binário httpgo; o compilador C é necessário para o
race detector do ambiente de testes.

## Portabilidade: build versus runtime

| Plataforma/arquitetura | Build CGO=0 | Execução nativa E2E | Ctrl+C real | TTY real |
|---|---|---|---|---|
| Linux amd64 | passou | passou, sem race | passou | passou via PTY |
| macOS amd64 | passou, cross-build em Linux | **pendente** | **pendente** | **pendente** |
| Windows amd64 | passou, cross-build em Linux | **pendente** | **pendente** | **pendente** |

Os três arquivos em `bin/portable/` são evidência de build, ignorados pelo git;
não são distribuição/release da US-19. Não foi anunciada execução em arquiteturas
adicionais. Ausência de ambiente nativo é uma lacuna de aceite, não uma conclusão.

Para comprovar runtime, executar `make e2e`/`go test ./tests/e2e -count=1` na
plataforma correspondente. O teste de sinal real existente tem tags linux/darwin;
Windows ainda exige harness de console/sinal nativo. O teste de PTY existente é
Linux-only. Um teste de contexto ou cross-build **não** preenche essas lacunas.

## Continuação e rastreabilidade

Plano: issue #4, tasks back:1 a back:5, sem frontend/estáticos. Os commits locais
são consultáveis com `git log --oneline`; o corpo identifica a validação parcial.
A fila não recebeu done final, portanto não disparou review nem comentário de
conclusão na issue. Faltam resolver os bloqueios acima antes da entrega.
