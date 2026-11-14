# AGENTS.md

Guia para agentes de código (e pessoas) neste projeto. Leia antes de alterar qualquer coisa.
Em caso de dúvida, prefira a solução mais simples e idiomática.

## 1. Visão geral

`httpgo` é uma CLI **independente** em Go para testes HTTP, compatível com um **subconjunto documentado** do formato Hurl (`.hurl`), **sem reutilizar o código-fonte do Hurl**.

- Preserve, sempre que suportadas, as convenções do Hurl para *request-line*, resposta esperada, `[Captures]` e `[Asserts]`.
- Extensões próprias e explícitas (fora do Hurl): **configuração YAML hierárquica** e **interpolação em arquivos externos**. Sempre marcadas como extensões na documentação.
- `docs/` é a fonte da verdade do subconjunto: o que não está documentado como suportado **não é suportado**.
- Go conforme `go.mod` (não aumentar sem necessidade). Binário único, sem dependências de runtime.

### Fatias verticais

Cada história é uma fatia vertical funcional, de ponta a ponta:

```
parsing → execução HTTP real → resultado na CLI → testes automatizados → documentação
```

A primeira história já executa `GET` e valida status. Não entregue "só o parser" ou "só o transport": cada história deve ser executável pela CLI e verificável pelo usuário, sem quebrar as anteriores.

### Nada é ignorado em silêncio

**Nenhuma construção reconhecida pode ser silenciosamente ignorada.**

- Construção reconhecida mas ainda não suportada (seção, opção, filtro, query, predicado): **falhar com erro explícito** informando arquivo, linha/coluna e a construção. Nunca executar o arquivo "parcialmente".
- Sintaxe inválida ≠ não suportado: use mensagens distintas.
- Todo `default`/`else` de `switch` sobre construções da linguagem retorna erro, nunca descarta o caso.
- Cada construção não suportada tem um cenário de teste em `testdata/scenarios/` comprovando o erro.

### Compatibilidade e independência

- **Não copie, traduza nem adapte código-fonte do Hurl.** Implemente a partir da documentação pública e do comportamento observável.
- Divergências propositais ficam registradas em `docs/` (seção "diferenças em relação ao Hurl").
- `testdata/compatibility/` é contrato: alterar um arquivo ali é mudança de contrato e deve ser sinalizada no PR.

## 2. Estrutura e dependências

```
cmd/httpgo/main.go   # ponto de entrada fino: monta dependências, chama o root command, mapeia erro → exit code
internal/
  cli/         # comandos, flags, stdout/stderr, exit codes
  parser/      # arquivo .hurl → modelo de domínio + validação
  runner/      # orquestra a execução dos passos e o estado entre eles
  transport/   # cliente HTTP (TLS, timeouts, retries), atrás de interface
  template/    # resolução de variáveis/interpolação
  query/       # extração de valores das respostas
  assertion/   # avaliação de asserções
  config/      # carga e validação (flags > HTTPGO_* > ambiente YAML > YAML base > defaults)
  report/      # renderização dos resultados (texto, JSON, ...)
testdata/
  scenarios/ payloads/ compatibility/
docs/          # subconjunto suportado, extensões, diferenças em relação ao Hurl, exemplos
```

Fluxo: `cli` → `config` → `parser` → `runner` → (`template`, `transport`, `query`, `assertion`) → `report`.

Regras de dependência:
- Sem ciclos. Pacotes folha (`template`, `query`, `assertion`, `transport`) não importam `runner`, `cli` nem `report`.
- Só `cli` conhece `cobra`/flags; só `transport` conhece `net/http`.
- `runner` depende de interfaces próprias (`Transport`, `Renderer`...), implementadas pelos demais pacotes e montadas em `main`/`cli`.
- `query` extrai valores; **quem decide sucesso/falha é `assertion`**. `report` apenas renderiza, não reavalia.
- Tipos do modelo de domínio ficam no pacote que os define (ex.: `parser`); nada de pacote genérico `types`/`utils`/`common`.
- Use `internal/` por padrão; não crie `pkg/` sem consumidor real.

## 3. Comandos

```bash
make build   # go build -trimpath -o bin/httpgo ./cmd/httpgo
make test    # go test ./... -race -cover
make lint    # golangci-lint run
make fmt     # gofmt -s -w . && goimports -w .
go mod tidy  # após mudar dependências
```

Antes de finalizar qualquer tarefa, rode `make fmt lint test`. Em cada entrega com alteração de código, `go vet ./...` também é obrigatório: execute-o explicitamente, salvo se estiver comprovadamente coberto pelo lint. Não entregue código que não compila ou com testes falhando.

## 4. Convenções de código

Siga *Effective Go*, *Go Code Review Comments* e o *Google Go Style Guide*. Pontos específicos aqui:

- Interfaces pequenas, definidas **no consumidor**; não crie interface sem segunda implementação ou necessidade de teste.
- Injeção por construtores (`NewX(deps)`). Sem estado global mutável nem `init()` com efeito colateral.
- Erros: contexto com `fmt.Errorf("...: %w", err)`, `errors.Is/As`, mensagens em minúsculo sem pontuação final. Sem `panic` em fluxo normal. Não logue **e** retorne o mesmo erro; trate na borda (`cli`).
- `context.Context` como primeiro parâmetro em I/O; propague cancelamento (`signal.NotifyContext`) e timeouts. Toda goroutine tem dono e condição de término (`errgroup`).
- Logging com `log/slog`, em **stderr**, atrás de `--verbose`/`--debug`. Nunca logue segredos.
- Biblioteca padrão primeiro; dependência nova só com ganho claro e manutenção ativa. Rode `govulncheck ./...` periodicamente.
- Linters em `.golangci.yml` (versionado). `//nolint` só com comentário justificando.

## 5. Contrato da CLI

- **stdout** = resultado (parseável); **stderr** = mensagens, progresso, erros e logs.
- `--output` seleciona o formato do resultado (ex.: `json` ou `junit`, quando implementados); `--output-file` seleciona o arquivo de destino. Sem arquivo de destino, o resultado vai para stdout. Formatos publicados são contrato estável.
- Respeite `NO_COLOR`; sem cores/animações quando stdout não for TTY. Injete `io.Reader`/`io.Writer` nos comandos.
- Flags longas sempre; `--help` com exemplos reais; `--version` (versão/commit/data via `-ldflags`).
- Precedência de configuração: **flags > env (`HTTPGO_*`) > ambiente YAML selecionado > YAML base > defaults**. `HTTPGO_*` se limita a opções explicitamente documentadas; não há mapeamento indiscriminado do ambiente do processo. Referências explícitas `${NOME}` são resolvidas nos valores efetivos após merge e overrides, não constituem uma camada adicional de precedência. Valide tudo na inicialização e falhe rápido.
- Valores secretos devem vir do ambiente do processo ou de stdin, nunca de flags, inclusive `--var`. `--secret <nome>` apenas marca um nome como protegido, sem fornecer seu valor. A entrada de segredos por stdin precisa ser especificada, implementada, testada e documentada antes de ser anunciada como suportada. Documente os limites de detecção de segredos arbitrários.
- Exit codes: `0` sucesso; `1` somente falha de asserção; `2` demais erros, incluindo uso, entrada, configuração, captura, renderização, transporte e escrita de relatório. Em uma suíte, `2` prevalece sobre `1`. Documente qualquer código adicional. `os.Exit` só em `main` (ele não executa `defer`).
- Erros dizem **o quê**, **por quê** e, se possível, **como resolver**; sem *stack trace* fora de `--debug`.
- Funcione em Linux, macOS e Windows: `filepath`, `os.UserConfigDir()`, sem suposições de shell.

## 6. Testes

- Teste **comportamento**, não implementação. Rápidos, determinísticos, independentes. Bug corrigido ganha teste de regressão.
- *Table-driven* com `t.Run`; `t.Helper()`, `t.Cleanup()`, `t.TempDir()`, `t.Setenv()`; `t.Parallel()` quando não há estado compartilhado. Mensagens de falha mostram entrada, obtido e esperado.
- Escolha **uma** abordagem de comparação (`go-cmp` ou `testify`) e mantenha-a.
- Nada de `time.Sleep` para sincronizar; injete relógio/`now`. Sem testes *flaky*: corrija a causa, não adicione retry.
- `go test ./... -race -cover` passa local e no CI. Cobertura é indicador, não meta: priorize caminhos de erro e regras críticas.
- Por pacote:
  - `transport`: `httptest.Server`.
  - `runner`: `Transport` falso, sem rede real.
  - `cli`: root command com `SetOut/SetErr/SetArgs`, sem executar o binário.
  - e2e: `testscript` (ou `os/exec`) contra o binário, validando stdout, stderr e **exit code**.
- Use fakes escritos à mão antes de geradores de mock. Testes com serviços reais ficam atrás de `//go:build integration`.
- `testdata/`: `scenarios/` (entradas para parser/runner/e2e, **incluindo inválidas e não suportadas**), `payloads/` (corpos reutilizados), `compatibility/` (pares entrada/saída de referência). Arquivos pequenos, nomeados pelo comportamento (`scenario_missing_url.hurl`), sem dados reais ou segredos. Golden files com flag `-update`; revise o diff no PR.
- Sempre cubra: `--help`, `--version`, flags inválidas, ausência de argumentos, saída com e sem TTY.
- Fuzzing (`go test -fuzz`) para `parser` e `template`.

## 7. Commits

Formato obrigatório: `<tipo>(<num_historia>): <descrição>`

- Tipos: `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `perf`, `build`.
- `<num_historia>`: número da história/card (ex.: `1234`). Se não souber, **pergunte**; não invente.
- Descrição no imperativo, minúscula, sem ponto final, até ~72 caracteres.
- Corpo opcional para o **porquê**; `BREAKING CHANGE:` no rodapé quando houver.

```
feat(1234): adiciona execução de GET com validação de status
fix(1301): corrige exit code quando a flag --output é inválida
test(1234): cobre construção não suportada com erro explícito
```

Um commit = uma mudança lógica, com build e testes passando. Não misture refatoração com mudança de comportamento. Nunca commite segredos nem `bin/`.

## 8. Definição de pronto e PR

Uma história só está pronta quando **todos** os itens são cumpridos:

- [ ] **Parsing**: nova construção lida e validada, com erros claros (arquivo, linha, coluna)
- [ ] **Execução HTTP real**: `runner`/`transport` executam de fato (testado com `httptest.Server`)
- [ ] **Resultado na CLI**: saída e exit code corretos, inclusive em falha
- [ ] **Testes**: unitários, e2e/golden e casos negativos (inválido e não suportado)
- [ ] **Documentação**: `docs/` atualizado (o que passou a ser suportado, exemplo mínimo, o que continua não suportado; extensões marcadas como tal)
- [ ] Nenhuma construção reconhecida é ignorada em silêncio
- [ ] `make fmt lint test` passam, com testes usando `-race -cover`; `go vet ./...` passa explicitamente ou está comprovadamente coberto pelo lint; `go mod tidy` se as dependências mudaram
- [ ] Contrato (flags, formato de saída, exit codes) inalterado, ou sinalizado como *breaking*

## 9. Segurança

- Nunca logue nem imprima tokens/senhas; mascare-os se precisar exibir (ex.: headers de autorização em relatórios).
- Valide toda entrada (arquivos, flags, env, respostas de rede); cuidado com *path traversal* ao resolver arquivos externos e na interpolação.
- Arquivos com credenciais `0600`, diretórios `0700`. TLS verificado por padrão; `InsecureSkipVerify` só por opção explícita do usuário.
- Sem `exec.Command` com string concatenada nem `sh -c` com entrada do usuário.

## 10. Instruções para agentes

**Faça:** leia o código existente e siga as convenções; mudanças pequenas e focadas; escreva e rode testes junto com o código; pergunte quando houver ambiguidade (número da história, formato de saída).

**Não faça:**
- Não ignore, descarte ou "aceite sem efeito" construções `.hurl` reconhecidas e não suportadas: falhe com erro explícito.
- Não copie nem adapte código do Hurl; não declare suporte ao que não esteja implementado, testado e documentado.
- Não entregue história parcial (sem execução real, testes ou docs).
- Não apresente extensões (YAML hierárquico, interpolação externa) como parte do formato Hurl.
- Não adicione dependências, mude a versão do Go ou altere CI/lint sem necessidade explícita.
- Não mude contrato público silenciosamente; não use `panic`, globais mutáveis, `init()` com efeito colateral ou `time.Sleep` em testes.
- Não reformate código não relacionado à tarefa nem desabilite linters para "fazer passar".
- Não faça commit/push por conta própria sem ser solicitado.
