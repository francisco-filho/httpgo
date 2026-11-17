# Validação — US-01 a US-16, US-20 a US-27

## US-26 — issue #50 / história 0026

Plano da issue #50 executado sem planner, somente backend, na ordem da fila
0026: `back:1` em `ed3eeab` (polling ponta a ponta, validação, resultado e métricas),
`back:2` em `93b3376` (cancelamento, replay, sigilo, prazos e recursos), `back:3`
nesta alteração (corpus, ajuda/documentação e validação integrada). Sem
frontend/estáticos, alteração de dependências/toolchain, CI/lint ou transporte
concreto de produção. Nenhuma fonte Hurl copiada/adaptada/consultada. Gates locais
não substituem review; backdev não executou análise SonarQube.

### Ambiente e execução real

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports`/`golangci-lint` existentes em
`/home/ff/go/bin`. Hurl **8.0.0** selecionado pelo PATH em
`/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, sem substituir
Hurl padrão 8.0.1 nem instalar ferramentas. O aviso libcurl de metadados não
impediu a identificação nem a execução dos pares.

- Tasks 1/2: passaram os testes específicos de parser/runner/CLI/suite e
  runner/CLI/suite/transport, respectivamente; em ambas passaram
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e `make build`
  antes do commit. Testes usam espera privada controlada, contextos/canais,
  fakes de executor/fontes/writers e HTTP local, sem sleeps de sincronização.
- Task 3: `make compatibility` passou com **283 pares**, Hurl **8.0.0** obrigatório,
  sem skip nem `-update` no gate. **10 novos pares `polling_*`** observados com
  filtro restrito; **273 pares anteriores inalterados**, sem regeneração global.
  Sequência é reiniciada entre executáveis. `polling_cli_zero` comprovou que
  `retry` local prevalece sobre `--retry 0` no Hurl; `decode`/`capture` registraram
  retry no Hurl contra encerramento 2 sem retry no HTTPGo. JSON/header/bytes,
  ordem e métodos são observados; duração e stdout não são comparados.
- Validação integrada passou: `make fmt lint test` (**0 issues**, race/cobertura),
  `go vet ./...`, `go build ./...`, `make build`, `make compatibility` e `make e2e`.
  Coberturas: assertion **94,2%**, CLI **97,5%**, config **96,0%**, parser **97,2%**,
  query **95,8%**, runner **94,4%**, suite **95,5%**, template **95,7%**,
  transport **90,4%**. Binário/E2E verifica stdout, stderr, exit code e HTTP real.
- Seeds do parser ampliados para opções, limites, duplicações, negativos,
  associação, multipart/JSON e LF/CRLF. Passaram os comandos:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**45.132 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**18.667 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**24.056 execuções**), sem crash. Contagens são somente desta rodada.
- `TestPollingHelpAndPublishedExample` executou ajuda root/test e o cenário
  versionado com intervalo 500 ms e duas divergências antes de READY. Sem
  asserção de duração/tolerância temporal; defaults/zero são verificados por
  observação da espera injetada, não por sleeps.
- Cross-builds e runtime macOS/Windows não executados: opcionais pelo plano v4,
  não bloqueios. Corpus sintético não é homologação da suíte real.

### Rastreabilidade GO-304–GO-323

| GO | Evidência |
|---|---|
| 304–305 | `TestPollingClassification`, `TestPollingBudgetAndInterval`, `TestPollingByBinary`, pares `ready`/`exhausted`/`zero`/`omitted`: recuperação/esgotamento, inicial + retry, sem próxima entrada em falha |
| 306 | `TestPollingClassification`: status, header, JSON, ausência/null elegíveis; decode/percurso são erros 2 |
| 307 | `TestPollingPostCapturesAndLogicalSequence`, `TestPollingByBinary`, par `post`: body/header completos e mesmos valores de entrada |
| 308–309 | `TestPollingCaptureAndPreparationErrors`, `TestPollingExternalFilesRereadAndPreparationMetrics`, fluxo POST/capturas: divergências não avaliam captura ausente, aprovação publica uma vez, erro 2 não repete |
| 310–311 | `TestEntryOptions`, `TestInvalidEntryOptionsBeforeExecution`, `TestPollingBudgetAndInterval`, negativos CLI/binário e `invalid_late`: associação, LF/CRLF, defaults, zero, limites e arquivo inteiro |
| 312 | tabelas de polling, `TestPollingCLIAndSuite`, `TestPollingSuiteMetricsAndFailFast`: resultado/callback único, contagens lógicas sem triplicação e tentativas reais; erro de preparo não soma envio |
| 313 | `TestPollingDiscardedResponseProtection`, `TestPollingDiscardedSecretProtectsFollowingFile` mais regressões de proteção existentes: segredo descartado permanece protegido em resultado/callback/streams e outro arquivo |
| 314 | fluxo POST/capturas e suíte: entrada anterior não repete, entrada posterior só começa após aprovação e callback terminal |
| 315 | `TestPollingExternalFilesRereadAndPreparationMetrics`, `TestPollingReopensMultipartBeforeWaiting`, `TestPollingReopensPayloadByBinary`: conteúdo modificado observado, caminho relativo preservado, sem snapshot |
| 316 | `TestPollingSuiteMetricsAndFailFast`, CLI/suíte e regressões da suíte: fail-fast só no desfecho, agregação real separada e prioridade 2 > 1 |
| 317 | 10 pares `polling_*`, `make compatibility` com 283 pares e `TestPollingHelpAndPublishedExample`; limites/diferenças documentados |
| 318 | `TestPollingCancellationAndTimer`: antes da inicial, durante espera e corrida timer/cancelamento, sem novo envio; timer cancelável e quantidade efetiva |
| 319 | `TestPollingEachAttemptHasFreshDeadline`, `TestPollingIncompleteOrTimedOutResponseDoesNotRetry`, `TestPollingIncompleteBodyStopsByBinary`: prazo novo por tentativa, caller menor prevalece, leitura incompleta é 2 |
| 320 | `TestPollingReopensMultipartBeforeWaiting`, E2E de releitura e regressões multipart existentes: fechamento antes da espera/reabertura, incrementalidade e falhas de leitura/fechamento/transporte |
| 321 | `TestPollingReportFailureDoesNotStartNextEntry`, `TestPollingTerminalWriteFailure`: erro/short write em resultado/resumos termina 2 sem retry/reimpressão; próximas entradas bloqueadas |
| 322 | segundo Run no mesmo runner/cenário, suíte e regressões de isolamento: orçamento/capturas não vazam, cenário não é modificado, sem respostas descartadas em resultados |
| 323 | `TestPollingRedirectsStartAtOriginalRequest`: 302/307 reais, POST original em cada tentativa, GET/POST final conforme redirect; hops não contam como retries |

## US-27 — issue #52 / história 0027

Plano da issue #52 executado sem planner e sem replanejamento, somente backend,
na ordem das três tasks aprovadas na fila: `back:1` no commit `d883c3d`
(projeção tipada ponta a ponta), `back:2` no commit `4696e7f` (sigilo, isolamento
e recursos) e `back:3` nesta alteração (corpus, ajuda, documentação e validação
integrada). Sem frontend/estáticos, dependência nova, mudança de Go, CI/lint,
novos predicados ou alteração do transporte de produção. US-23 já presente na
base, reutilizada sem reimplementação. Nenhum código-fonte Hurl consultado,
copiado ou adaptado. Os gates locais não substituem o review.

### Ambiente e comandos efetivamente executados

Rodada final em **2026-11-17**, Linux amd64, Go `go1.26.10-X:nodwarf5`.
`goimports` e `golangci-lint` existentes em `/home/ff/go/bin`.
Hurl **8.0.0** inicialmente disponível em `/tmp/httpgo-0021-hurl`; após a
interrupção da sessão e limpeza de `/tmp`, o binário público da release fixada
foi extraído em `/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`
e selecionado no PATH somente para o diferencial. Hurl padrão 8.0.1 não foi
substituído. O aviso de metadados locais de libcurl não impediu identificação
ou execução do binário 8.0.0; não houve skip ou afrouxamento da versão no harness.

- Task 1: testes de `internal/parser`, `query`, `assertion`, `template`, `runner`
  e `cli`; E2E de binário/HTTP local; `make fmt lint test`, `go vet ./...` e
  `go build ./...` passaram antes do commit. O gate completo de testes incluiu
  os E2E com o diagnóstico de segunda assertion corrigido para a linha original.
- Task 2: passaram `go test ./internal/runner ./internal/cli ./internal/suite
  ./internal/transport`, `go test ./tests/e2e -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit. Fakes de read/close, HTTP
  local, canais/contextos e writers, sem sleeps ou serviços externos.
- Task 3: **dez novos pares `projection_*`**; os pares antigos
  `captures_unsupported` e `assert_json_query_unsupported` mudam intencionalmente
  de 2/zero envios para 0 com fluxo completo. `-update` ficou limitado a esses
  doze pares e as observações/classificações foram revisadas: Hurl interrompe
  reuso composto com 3; ramos incompatíveis são descartados pelo Hurl nos três
  pares estritos, mas continuam erros 2 aqui. Os outros **271 pares** não mudam.
  `make compatibility` passou com **283 pares** e Hurl **8.0.0**, sem `-update`.
- Gate final: passaram `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build`, `make compatibility` e `make e2e`. Lint: **0 issues**.
  `make test` executa `go test ./... -race -cover`. Coberturas: assertion **94,2%**,
  CLI **97,5%**, config **96,0%**, parser **97,5%**, query **96,3%**,
  runner **93,6%**, suite **95,4%**, template **96,1%**, transport **90,4%**.
  O gate interrompido não foi contado como conclusão; foi repetido após a retomada.
  A conferência final acrescentou proteção de resumo sintetizado (tipo/comprimento
  que coincida com segredo mesmo sem folhas secretas), com regressão
  `TestProjectionSynthesizedSummaryProtection`; os gates completos foram repetidos
  após a correção.
- Seeds ampliados com wildcards aninhados, índices, LF/CRLF, malformados e
  compostos projetados. Passaram sem crash:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**30.871 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**29.800 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**117.301 execuções**). Contagens específicas desta rodada.
- Ajuda root/test e exemplo publicado executados pelo binário com servidor HTTP
  local; cinco novos negativos versionados verificam rejeição antes de todos
  os envios. Negativos antigos de wildcard foram refinados para wildcard de
  objeto/slice, não removidos. `gofmt -l` dos arquivos Go alterados, diff e links
  locais novos conferidos.
- Cross-builds e testes nativos macOS/Windows não realizados (opcionais durante
  a história, conforme plano v4). Corpus sintético não homologa a suíte real e
  não comprova JSONPath completo nem equivalência geral com Hurl.

### Validação após rebase com develop

Rebase sobre `origin/develop` em `afd1063` (US-26/polling), preservando a
documentação e os seeds de fuzzing de ambas as histórias. Os hashes de tasks
acima refletem o histórico reescrito; os resultados anteriores registram a
rodada antes dessa integração.

Após resolver os conflitos, passaram os testes específicos de parser, query,
assertion, template, runner, CLI, suite e transport; `make fmt lint test`
(**0 issues**, race/cobertura), `go vet ./...`, `go build ./...` e `make e2e`.
`make compatibility` passou sem `-update` com **293 pares** e Hurl **8.0.0**:
os dez pares de polling da develop foram preservados, além dos dez de projeção.
Coberturas integradas: parser **97,2%**, runner **94,1%**, suite **95,5%**;
demais pacotes mantiveram os percentuais da rodada US-27 acima.
`FuzzParse` foi repetido com os seeds combinados, `-fuzztime=20s -parallel=2`,
passando com **12.167 execuções**, sem crash. Nenhum golden foi regenerado.

### Rastreabilidade GO-324–GO-343

| GO | Evidência executada |
|---|---|
| GO-324 | `TestProjectionByBinary`, `TestProjectionHelpAndPublishedExample`, `TestProjectionPredicatesAndOrder`: demo/reordenação 0, falta do requerido/proibido 1, sem captura/próximo envio |
| GO-325–328 | `TestProjectionCardinality`, `TestProjectionReducedTypesByBinary`, par `projection_cardinality`: wildcards aninhados, índice após wildcard, redução única, ordem/repetições, ramos ausentes, null presente e arrays sem flatten |
| GO-329–330 | `TestProjectionPredicatesAndOrder`, `TestProjectionReducedTypesByBinary`, par `projection_scalar_count`: count do valor reduzido, escalar/ausência não viram coleção; contains, igualdade, matches e limites sem aplicação implícita por nó |
| GO-331 | `TestProjectionCaptureReuse`, pares `projection_nested_capture`, `projection_arrays_capture`, `projection_empty_array_capture`, `projection_precision_capture`: publicação/reuso tipado, precisão/grafia numérica e template literal |
| GO-332–333 | `TestProjectionCaptureReuse`, `TestProjectionCaptureAtomic`, par `projection_missing_capture`: sem resultado, contextos textuais, conflito ancestral/descendente, proteção descoberta e ausência de publicação parcial/sobrescrita |
| GO-334 | `TestProjectionStrictTraversal`, `TestProjectionPredicatesAndOrder`, `TestProjectionCaptureAtomic`, três pares `projection_strict_*`: nenhum resultado parcial ou aprovação por negação em percurso incompatível |
| GO-335 | `TestProjectionMalformedPaths`, `TestProjectionDeclarations`, `TestProjectionInvalidLateNeverSends`, `TestProjectionStructuralCorpusByBinary`: LF/CRLF, posições originais e zero envios; regressão dos caminhos simples preservada |
| GO-336–337 | `TestProjectionStrictTraversal`, `TestProjectionPredicatesAndOrder`, regressões anteriores de captura/assertions: JSON vazio/malformado/adicional, decode sob demanda, primeira falha, captura só depois de aprovação |
| GO-338 | `TestProjectionSafeFailure`, `TestProjectionProtectedSummariesCLI`, `TestProjectionSuiteProtectionAndIsolation`, `TestProjectionReportsAndSessionsIndependent`, `TestProjectionSynthesizedSummaryProtection`: callbacks/resultados/streams sem coleção, esperado composto ou metadados/contagens protegidos, captura pendente não antecipada e proteção descoberta mantida após falha |
| GO-339 | `TestCompatibility` (283 pares, dez novos e dois contratos alterados), ajuda root/test e `scenario_projection.hurl` executáveis; diferenças deliberadas em `compatibility.md` |
| GO-340–341 | `TestProjectionFinalResponseAndCancellation`, `TestProjectionReadCloseBeforeReport`, `TestProjectionFinalAndIncompleteByBinary`: somente resposta final completa, read/close/timeout/cancel falham antes de seleção/captura/próximo envio; regressões de header isolado mantidas |
| GO-342 | `TestProjectionCallbackFailurePreservesCounts`, `TestProjectionWriterFailureOverridesOutcomes`: falha/short write com 2, sem retry/envio posterior, resultados/contagens anteriores preservados |
| GO-343 | `TestProjectionReportsAndSessionsIndependent`, `TestProjectionPublishedValuesDoNotCrossRuns`, `TestProjectionSuiteProtectionAndIsolation`: cenário/documento não mutados, planos removidos de resultados/callbacks, sessões e invocações independentes |


## US-24 — issue #46 / história 0024

Plano/arquitetura/tasks da issue #46 executados sem planner, somente backend,
na ordem da fila 0024: `back:1` no commit `720a197` (regex de header ponta a ponta,
pré-validação e diagnóstico protegido), `back:2` no commit `8885b74` (sigilo,
isolamento e recursos), `back:3` nesta alteração (corpus, ajuda, documentação e
gates integrados). Sem frontend/estáticos, mudança de Go/dependências, CI/lint,
query ou transporte de produção. Nenhuma fonte Hurl consultada/copiada/adaptada.
Os gates locais não substituem o review; SonarQube não foi executado pelo backdev.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`. Ferramentas existentes `goimports` e
`golangci-lint` selecionadas por `/home/ff/go/bin` no PATH. Hurl **8.0.0** existente
em `/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` selecionado no
PATH do diferencial, sem substituir Hurl padrão 8.0.1 ou instalar ferramentas.
Aviso local de metadados libcurl não impediu identificação/execução.

- Task 1: passaram `go test ./internal/parser ./internal/assertion ./internal/runner
  ./internal/cli`, `go test ./tests/e2e -count=1`, `make fmt lint test`,
  `go vet ./...` e `make build` antes do commit.
- Task 2: passaram `go test ./internal/runner ./internal/cli ./internal/transport`,
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e
  `make build` antes do commit. Recursos com HTTP local e fakes de read/close,
  canais/contextos sem sleeps/retries; resultados e callbacks inspecionados.
- Task 3: **30 novos pares `header_matches_*`** observados contra Hurl **8.0.0**.
  A primeira rodada explicitou diferenças de regex inválida e delimitador; somente
  fixtures novas foram corrigidas pontualmente. Headers presentes nas fixtures de
  regex inválida evitam confundir falha de ausência com compilação no Hurl.
  `make compatibility` passou com **243 pares**, sem skip/`-update`; os **213
  anteriores não foram alterados/regenerados**. Harness não precisou mudar.
- Gate integrado: passaram `make compatibility`, `make fmt lint test`,
  `go vet ./...`, `go build ./...`, `make build` e `make e2e`. Lint **0 issues**;
  `make test` usa **race e cobertura** em todos os pacotes. Coberturas:
  assertion **94,2%**, parser **97,4%**, query **95,6%**, runner **93,5%**,
  CLI **97,5%**, transport **90,4%**, template **95,7%**, config **96,0%**,
  suite **95,4%**. E2E reutiliza build único e `exec.CommandContext`, verificando
  stdout/stderr, exit code e contadores de HTTP real.
- Seeds do parser ampliados com header matches, LF/CRLF, escapes, placeholders,
  vazio e padrões inválidos/não suportados tardios. Passaram sem crash:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**62.484 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**17.241 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**32.772 execuções**). Contagens descrevem apenas esta rodada.
- Exemplo publicado e ajuda root/test exercitados pelo binário; seis negativos
  versionados verificam rejeição antes de qualquer envio. Formatação Go, diff e
  links locais das páginas alteradas conferidos.
- Cross-builds e execução nativa macOS/Windows não realizados: opcionais conforme
  plano v4, não bloqueios. Corpus sintético não é homologação da suíte real nem
  prova de equivalência geral de engines, gramática ou políticas de segurança.

### Rastreabilidade GO-272–GO-286

| GO | Evidência executada |
|---|---|
| GO-272 | `TestHeaderMatchesSessionByBinary`, `TestHeaderMatchesHelpAndPublishedExample`, pares `session`/`session_missing_secure`: sessões distintas, formato divergente/ausência, 0/1 e interrupção sem captura/próximo envio |
| GO-273 | `TestHeaderMatchesSemantics`, `TestHeaderMatchesSemanticsByBinary`, pares `partial`/`anchored`: busca interna versus âncoras |
| GO-274 | `TestHeaderMatchesParsing`, `TestHeaderMatchesSemantics`, `TestHeaderMatchesSemanticsByBinary`, pares `escapes`/`slash`/`groups`: JSON uma vez e regex depois; gramática antiga preservada |
| GO-275 | mesmos testes de semântica, pares `case`/`option`/`unicode`/`unicode_class`/`normalization`: caixa, opções Go e Unicode sem normalização; newline só no fake, sem relaxar HTTP |
| GO-276 | runner/CLI/E2E de matches e pares `empty_*`/`missing`: ausência distinta de vazio, padrão vazio e `^$` |
| GO-277 | `TestHeaderMatchesSemantics`, `TestHeaderMatchesCLI`, `TestHeaderMatchesSemanticsByBinary`, pares `first`/`second`/`first_empty`/`second_after_empty`/`comma`: nome sem caixa e primeira ocorrência inclusive vazia |
| GO-278 | `TestHeaderMatchesInvalidLate`, `TestHeaderMatchesInvalidPreflightCLI`, `TestHeaderMatchesInvalidLateByBinary`, `TestHeaderMatchesStructuralCorpusByBinary`: erro tardio 2, zero envios e diagnóstico localizado/seguro LF/CRLF; negativos de nomes/seção e operadores preservados |
| GO-279 | `TestHeaderMatchesSemantics`, `TestHeaderMatchesSemanticsByBinary`, pares `literal_missing`/`literal_defined`: padrão não resolve variável/captura ou exige definição |
| GO-280 | `TestHeaderMatchesOrderAndCaptures`: status primeiro, JSON/header na ordem, captura depois; falhas 1/2 distintas, reutilização após aprovação e HTTP * sem comparação numérica |
| GO-281 | `TestHeaderMatchesSensitiveFailure`, `TestPendingSecretConcealsAssertionsAndMetadata`, `TestHeaderMatchesKnownDerivedSecrets`, `TestHeaderMatchesCLI`, `TestJSONAndHeaderPendingSecretCLI`, `TestHeaderMatchesPendingSecretByBinary`: proteção antes de callbacks/coleção/streams, sem antecipar captura falha |
| GO-282 | `TestCompatibility` (30 pares novos, 243 totais, Hurl 8.0.0), `TestHeaderMatchesHelpAndPublishedExample`: matriz, ajuda e exemplo executáveis, diferenças explícitas |
| GO-283 | `TestHeaderMatchesCompleteFinalResponse`, `TestHeaderMatchesReadCloseAndDiscard`, `TestHeaderMatchesFinalAndIncompleteByBinary`: só resposta final completa; header isolado descarta body não JSON |
| GO-284 | mesmos testes de transporte/E2E e `TestHeaderMatchesConsumptionRealCancellation`: read/close/timeout/cancel/incomplete são 2, zero concluídas, recursos fechados e sem aprovação/captura/retry/próximo envio |
| GO-285 | `TestHeaderMatchesReportFailurePreservesCounts`, `TestHeaderMatchesWriterFailureOverridesAllOutcomes`, `TestHeaderMatchesRealOutputFailure`: write/short write prioritário, contagens/resultados anteriores preservados, sem envio extra ou repetição no writer falho |
| GO-286 | `TestHeaderMatchesMetadataAndIsolation`, `TestHeaderMatchesKnownDerivedSecrets`, callbacks de segurança: regex original reutilizável, nenhuma compilada em metadados de sucesso/falha; sessões e proteção dinâmicas independentes |

## US-25 — issue #48 / história 0025

Plano da issue #48 executado sem planner, somente backend, na ordem da fila
0025: `back:1` no commit `99f3f8b` (extração ponta a ponta, parser/query/runner,
sigilo e recorte inicial), `back:2` no commit `3c278c7` (regressões de falhas,
recursos e isolamento), `back:3` nesta alteração (corpus, ajuda, documentação e
gates integrados). US-21/22 estavam disponíveis na base. Sem frontend/estáticos,
nova dependência, alteração de Go, CI/lint, transporte de produção, interfaces,
concorrência ou fonte de segredos. Nenhuma fonte Hurl consultada/copiada/adaptada.
Esta evidência local não substitui review; backdev não executou SonarQube.

### Ambiente e comandos efetivamente executados antes do rebase

Os commits acima foram reaplicados sobre a US-24 (`develop` em `537ea01`).
A rodada inicial abaixo ocorreu antes desse rebase; a revalidação da base
integrada é registrada separadamente.

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports` e `golangci-lint` existentes
em `/home/ff/go/bin` expostos no PATH. Hurl **8.0.0** existente em
`/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` selecionado no PATH
somente para o diferencial, sem substituir Hurl padrão 8.0.1 nem instalar tools.
Aviso de metadados libcurl não impediu execução.

- Task 1: passaram `go test ./internal/parser ./internal/query ./internal/template
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/template ./internal/runner ./internal/cli
  ./internal/suite ./internal/transport`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
  Lint inicialmente apontou switch simplificável em teste novo: corrigido sem
  desabilitar regra, e gate completo executado novamente com sucesso.
- Task 3: **30 novos pares** `capture_regex_*` confrontados com Hurl **8.0.0**,
  fixtures independentes. `-update` restrito aos novos pares e ao único contrato
  antigo intencionalmente alterado: `capture_header_filter.json` passa de 2/zero
  envios a 0 com grupo literal enviado. Demais **212 pares antigos inalterados**.
  Resultado integrado: `make compatibility` passou com **243 pares**, sem skip
  ou update. Read/charset/headers reutilizam protocolo existente; somente Cookie
  recebido foi acrescentado à observação opcional, com dados fictícios.
- Gate integrado executado: `make compatibility`, `make fmt lint test`,
  `go vet ./...`, `go build ./...` e `make e2e`, todos com sucesso. Lint **0 issues**;
  testes com **race e cobertura**. Cobertura: parser **97,3%**, query **95,8%**,
  runner **93,7%**, CLI **97,5%**, transport **90,4%**, template **95,7%**,
  assertion **94,2%**, config **96,0%**, suite **95,4%**. E2E usa build único,
  `exec.CommandContext` e HTTP local; verifica streams, código e envios reais.
- Parser recebeu seeds regex de body/header, escapes, grupos e formas não
  suportadas/tardias em CRLF. Passaram sem crash, com limites de **20s** e
  **parallel=2**: `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$'`
  (**65.086 execuções**), `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'`
  (**36.225**) e `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$'`
  (**54.286**), todos com `-fuzztime=20s -parallel=2`. Contagens são desta rodada,
  não meta de cobertura.
- Ajuda root/test e os dois exemplos publicados são conferidos e executados
  pelo binário real. Corpus negativo exercita LF/CRLF e zero envios do arquivo.
  `gofmt -l` dos Go alterados, `git diff --check` e conferência dos destinos de
  links locais em README/subset/architecture/compatibility/validation passaram.
- Cross-builds e execução nativa macOS/Windows não realizados: opcionais conforme
  plano v4, não bloqueios. Corpus sintético não homologa suíte real nem prova
  equivalência geral de engines, charset, diagnóstico ou segurança com Hurl.

### Revalidação após rebase sobre a US-24

Por solicitação do responsável, a branch foi rebaseada sobre `develop` em
`537ea01`, que contém a US-24 (`f7ddfe5`). Mapeamento dos commits reescritos:
`81d4100` → `99f3f8b`, `8a429cd` → `3c278c7`, `7563cc9` → `aee9f6c`.
Conflitos em README, documentação e seeds foram resolvidos preservando ambas
as histórias; `header matches`, extração de capturas e seeds das duas histórias
permanecem ativos. Sem regenerar goldens para acomodar a integração.

Passaram novamente na base integrada:

- `go test ./internal/parser ./internal/query ./internal/template ./internal/runner ./internal/cli`.
- `make compatibility`: **273 pares**, Hurl **8.0.0**, sem skip ou update.
  Dos **243 pares da base US-24**, somente `capture_header_filter.json` muda
  intencionalmente na US-25; os outros **242** permanecem inalterados.
- `make fmt lint test` (lint **0 issues**, race/cover), `go vet ./...`,
  `go build ./...` e `make e2e`. Cobertura do parser integrado: **97,5%**;
  demais percentuais iguais aos registrados na rodada inicial da US-25.
- `TestHeaderMatchesBeforeRegexCaptureByBinary` combina US-24/25 na mesma
  resposta: assertion sensível antes da captura mesmo com seções invertidas;
  somente token no Cookie explícito, body indecodificável não consultado,
  mismatch impede próxima requisição e regex inválida tardia impede todo HTTP.
- Fuzzers repetidos com `-fuzztime=20s -parallel=2`: `FuzzParse` **27.290**,
  `FuzzRender` **17.407**, `FuzzRenderJSON` **53.127** execuções, todos sem crash.
- `git diff --check`, `gofmt -l` dos Go alterados e destinos dos links locais
  conferidos. Nenhum push nem análise SonarQube realizado pelo backdev.

### Rastreabilidade GO-287–GO-303

| GO | Evidência executada |
|---|---|
| GO-287 | CLI `TestRegexCaptureCLI`, E2E `TestRegexCaptureOTPAndCookieByBinary`, exemplo `scenario_capture_otp.hurl`, par `otp`: string exata protegida, ausência impede POST |
| GO-288 | mesmo E2E nos modos cookie/no implicit cookie, exemplo `scenario_capture_refresh.hurl`, pares `cookie`/`no_cookie_store`: somente token em Cookie explícito |
| GO-289 | query `TestRegexFirstGroup`, runner `TestRegexCaptureSelectionAndPrecedence`, par `first_group`: primeiro grupo da primeira correspondência, sem correspondência inteira |
| GO-290 | query/runner acima, runner `TestRegexCaptureStringsAndStaticPatterns`, pares `empty`, `optional`, `header_empty`: vazio participante válido, opcional ausente falha |
| GO-291 | parser `TestRegexCaptureStaticValidation`, runner de seleção/precedência, pares `no_group`/`no_match`: sem grupo aceita parsing e falha na captura |
| GO-292 | runner `TestRegexCaptureStringsAndStaticPatterns`, pares `number`, `boolean`, `json`, `placeholder`, `escapes`, `static_defined`, `static_missing`: strings e padrão estático, sem recursão |
| GO-293 | runner de seleção, pares `utf8`, `latin1`, `invalid_utf8`, `unknown_charset`; US-21 decoder reutilizado sem normalização/fallback |
| GO-294 | runner de strings/seleção, pares `header_*`, `cookie`: caixa, primeira ocorrência, vírgula, ausência/vazio e independência do decoder de body |
| GO-295 | parser `TestRegexCaptureModelAndPositions`/`TestRegexCaptureStaticValidation`, CLI de sintaxe tardia, E2E `TestRegexCaptureStructuralCorpusByBinary`: localização LF/CRLF, zero envios e diagnósticos sem padrão |
| GO-296 | runner `TestRegexCaptureSelectionAndPrecedence`/`TestRegexCaptureProtectionAndAtomicityOnFailure`, pares `status_first`/`assertion_first`: status/assertions antes de captura |
| GO-297 | runner `TestRegexCaptureAtomicMixedBatches`/`TestRegexCaptureProtectionAndAtomicityOnFailure`, pares `mixed`/`collision`: lote atômico, iniciais/anteriores/lote não sobrescritos |
| GO-298 | runner de proteção com sessão observável/callbacks, CLI `TestRegexCaptureWritersAndDerivedProtection`, E2E `TestRegexCaptureFinalAndFailureByBinary`: grupo protegido imediatamente, inclusive lote falho/pending, resultado sem dados operacionais |
| GO-299 | 30 pares novos/273 totais após integração da US-24, `TestRegexCaptureHelpAndPublishedExamples`, matriz revisada: diferenças de tipagem/ordem/pré-validação/charset/headers explícitas |
| GO-300 | transport `TestRegexCapturesRequireCompleteFinalResponse`/`TestRegexCaptureConsumesAndClosesBeforeEvaluation`, E2E final/failure: final completa, retenção só no body |
| GO-301 | mesmos testes de transporte nos modos read/close/incomplete/timeout/cancel: 2, sem próxima requisição/retry e zero concluídas em falha de consumo |
| GO-302 | runner de proteção modo report e CLI writers: erro/short write prevalece, preserva contagens, sem reimpressão/resumo no writer falho nem próximo envio |
| GO-303 | runner `TestRegexCaptureReusedRunnerAndEarlyReferences`, CLI `TestRegexSuiteProtectionWithoutResolutionSharing`: imutabilidade, referência prematura, nova sessão/invocação; suíte só acumula proteção, não resolução |


## US-23 — issue #44 / história 0023

Plano/arquitetura/tasks da issue #44 executados sem planner, somente backend,
na ordem da fila 0023: `back:1` no commit `bbe82de` (predicados delimitados,
proteção e testes ponta a ponta), `back:2` no commit `8df0444` (regressões de
recursos, proteção e isolamento), `back:3` nesta alteração (corpus, ajuda,
documentação e validação integrada). Sem frontend/estáticos, dependências novas,
aumento de Go, alteração de CI/lint ou transporte de produção. `internal/query`
permanece intacto. Nenhuma fonte Hurl foi consultada/copiada/adaptada.
Os gates locais não substituem o review; SonarQube não foi executado pelo backdev.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`. Ferramentas existentes `goimports` e
`golangci-lint` expostas por `/home/ff/go/bin` no PATH. Hurl **8.0.0** existente
em `/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` selecionado pelo
PATH do diferencial; Hurl padrão 8.0.1 não foi substituído. Nenhuma ferramenta
foi instalada. Aviso local de metadados da libcurl não impediu execução.

- Task 1: passaram `go test ./internal/parser ./internal/query ./internal/assertion
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test` e `go vet ./...` antes do commit.
- Task 2: passaram `go test ./internal/runner ./internal/cli ./internal/transport`,
  `go test ./tests/e2e -count=1`, `make fmt lint test` e `go vet ./...`
  antes do commit. Recursos testados com fakes de leitura/close e HTTP local,
  sincronização por canais/contextos, sem sleeps, retries ou serviço externo.
- Task 3: **37 pares novos** `membership_*` confrontados com Hurl **8.0.0**.
  Observações novas corrigidas pontualmente após execução real: header repetido
  negado segue pertencimento em lista no Hurl; `numbers` usa `123.0` para isolar
  decimais, mantendo `exponent` como divergência de gramática. O gate
  `make compatibility` passou com **213 pares**, sem skip ou update. Nenhum dos
  **176 pares anteriores** foi alterado/regenerado; harness não foi ampliado.
- Gate integrado: passaram `make compatibility`, `make fmt lint test`,
  `go vet ./...`, `go build ./...` e `make e2e`. Lint **0 issues**;
  `make test` executa todos os pacotes com **race e cobertura**. Cobertura:
  assertion **94,2%**, parser **96,6%**, query **95,6%**, runner **93,4%**,
  CLI **97,5%**, transport **90,4%**, template **95,7%**, config **96,0%**,
  suite **95,4%**. E2E reutiliza build único e `exec.CommandContext` contra
  servidores locais, verificando stdout, stderr, exit codes e envios reais.
- Seeds do parser ampliados para predicados novos, LF/CRLF, tipos e formas
  inválidas/não suportadas. Passaram, sem crash:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**32.547 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**33.282 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**26.061 execuções**). Contagens descrevem somente esta rodada, não uma meta.
- `gofmt -l` dos arquivos Go da história e `git diff --check` sem apontamentos;
  links locais de README/subset/architecture/compatibility/validation conferidos.
  Exemplo publicado e ajuda root/test exercitados por binário real.
- Cross-builds e execução nativa macOS/Windows não realizados: opcionais conforme
  plano v4, não bloqueios desta história. Corpus sintético não é homologação da
  suíte real nem prova de equivalência geral de segurança/gramática com Hurl.

### Rastreabilidade GO-255–GO-271

| GO | Evidência executada |
|---|---|
| GO-255 | runner `TestMembershipDemonstration`, E2E `TestMembershipDemonstrationByBinary` e `TestMembershipHelpAndPublishedExample`: demonstração/alterações 0/1, sem captura/envio posterior em falha |
| GO-256 | assertion `TestJSONMembership`, runner `TestMembershipLiteralAndHeaderSelection`, par `literal`, E2E `TestMembershipByBinary`: caixa, espaços, escapes, Unicode e placeholders estáticos |
| GO-257 | assertion `TestJSONMembership`, E2E `TestMembershipByBinary`, pares `typed`, `numeric_string`, `nonrecursive`: pertencimento escalar tipado, null e sem recursão |
| GO-258 | `TestJSONMembership`, `TestMembershipByBinary`, pares `numbers`, `precision`, `exponent`: números equivalentes/distintos, zero, decimais e expoentes sem expansão |
| GO-259 | `TestJSONNotExists`, runner `TestMembershipOrderAndPresence`, E2E `TestMembershipByBinary`, pares `missing`, `index_missing`, `null_present`, `root_present`: ausência distinta de presença |
| GO-260 | `TestJSONMembership`, `TestMembershipOrderAndPresence`, `TestMembershipCLI`, cinco pares `incompatible_*`: incompatibilidade não invertida em aprovação |
| GO-261 | `TestJSONMembership`, `TestMembershipByBinary`, pares `empty_array`, `empty_substring`, `empty_element`: fronteiras de vazio |
| GO-262 | `TestHeaderNotContains`, `TestMembershipLiteralAndHeaderSelection`, pares `header_*`: nome sem caixa, primeira ocorrência, ausente/vazio/vírgula |
| GO-263 | parser `TestMembershipInvalidLate`, CLI `TestMembershipInvalidCLINeverSends`, E2E `TestMembershipInvalidNeverSends` e `TestMembershipStructuralCorpusByBinary`, sete negativos versionados: erro tardio impede todo HTTP, inclusive LF/CRLF |
| GO-264 | `TestMembershipOrderAndPresence`, `TestMembershipDemonstration`, pares `header_first_order`, `json_first_order`: status/assertions/capturas e primeira falha preservados, inclusive HTTP * |
| GO-265 | `TestMembershipOrderAndPresence`, `TestMembershipCLI`, `TestMembershipByBinary`, par `traversal`: vazio/malformado/adicional/percurso incompatível são 2, nunca ausência |
| GO-266 | `TestMembershipSafeFailures`, `TestMembershipDerivedSecretsAndPendingMetadata`, `TestMembershipSecretOperandsCLI`: arrays/objetos e headers sensíveis não vazam; operandos/metadados conhecidos/derivados e captura pendente protegidos, sem avaliar captura falha |
| GO-267 | 37 novos pares e 213 totais em `TestCompatibility`, `TestMembershipHelpAndPublishedExample`, matriz explícita em `compatibility.md`, exemplo em README/subset e arquitetura |
| GO-268 | transport `TestMembershipFinalResponseAndCancellation`, `TestMembershipRequiresCompleteClosedResponse`, E2E `TestMembershipFinalAndIncompleteByBinary`: somente final completa; header isolado descarta body sem exigir JSON |
| GO-269 | mesmos testes de transporte nos modos read/close/timeout/cancel/incomplete; E2E timeout/incomplete: erro 2, zero concluídas, sem captura/envio extra/retry |
| GO-270 | `TestMembershipCallbackFailurePreservesEarlierResults`, CLI `TestMembershipWriterFailureOverridesAllOutcomes`, E2E Linux `TestMembershipRealOutputFailure`: write/short write prioritário, sem reimpressão/envio seguinte e contagens anteriores preservadas |
| GO-271 | `TestMembershipSessionsAndReportsStayIndependent`, `TestMembershipDerivedSecretsAndPendingMetadata`, callbacks de segurança: cenário/contexto reutilizáveis, sessão nova, sem documento/coleção/esperado bruto em relatório |


## US-22 — issue #42 / história 0022

Plano da issue #42 executado sem planner, somente backend e na ordem da fila
0022: `back:1` no commit `eb31916` (captura literal, sessão/proteção, testes e
recorte inicial), `back:2` no commit `a06bd7c` (segurança, recursos e falhas),
`back:3` nesta alteração (corpus, ajuda, documentação e validação integrada).
Sem frontend/estáticos, fonte Hurl, dependências novas, alteração de Go,
transporte de produção ou configuração de CI/lint. A implementação reutiliza
`query.Header`, lote tipado e sanitização existentes. Esta evidência local não
substitui o gate de revisão; nenhuma análise SonarQube foi executada pelo backdev.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`. `goimports` e `golangci-lint` existentes
em `/home/ff/go/bin` foram antepostos ao PATH. Hurl **8.0.0** já disponível em
`/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` foi selecionado
somente pelo PATH do diferencial; o Hurl padrão 8.0.1 não foi substituído.
Nenhuma ferramenta foi instalada. Aviso de metadados da libcurl não impediu
execução. Não foi consultado código-fonte Hurl.

- Task 1: passaram `go test ./internal/parser ./internal/query ./internal/template
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/template ./internal/runner ./internal/cli
  ./internal/transport`, `go test ./tests/e2e -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit.
- Task 3: **18 novos pares** gerados com `-update` limitado a `capture_header_*`,
  inspecionados quanto a códigos, fluxo, headers e bytes JSON. O gate
  `make compatibility` passou com **176 pares**, Hurl **8.0.0**, sem skip e sem
  update. Nenhum dos **158 pares anteriores** foi alterado/regenerado.
  `TestCompatibilityBodyFixture` também passou com a tag compatibility.
- Gate final: `make fmt lint test`, `go vet ./...`, `go build ./...`, `make e2e`
  e `make compatibility` passaram. Lint **0 issues**; testes com **race e cobertura**.
  Cobertura observada: parser **96,5%**, query **95,6%**, template **95,7%**,
  runner **93,4%**, CLI **97,5%**, transport **90,4%**, assertion **92,7%**,
  config **96,0%**, suite **95,4%**. E2E usa build único, servidores HTTP locais
  com portas dinâmicas, stdout/stderr e exit codes, sem rede externa/dummy.
- Fuzzing com seeds de headers/capturas ampliados e marcas dinâmicas na sessão:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**76.264 execuções**), `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'
  -fuzztime=20s -parallel=2` (**46.232**) e `go test ./internal/template -run '^$'
  -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2` (**35.717**) passaram sem crash.
  Contagens são apenas desta execução, não meta de cobertura.
- `gofmt -l` dos **21 arquivos Go alterados** e `git diff --check` sem
  apontamentos; links Markdown locais de README/subset/compatibility conferidos.
- Builds cruzados e execução nativa macOS/Windows não realizados: opcionais
  durante esta história conforme plano v4. O corpus sintético não é homologação
  da suíte real. Proteção própria HTTPGo não é equivalência de segurança Hurl.

### Rastreabilidade dos cenários

| GO | Evidência executada |
|---|---|
| GO-240 | `TestHeaderCaptureCacheByBinary` executa `scenario_capture_cache.hurl` com três caixas de ETag; `capture_header_cache` observa `If-None-Match` e 304 em ambos os binários |
| GO-241 | `TestHeaderCaptureLiteralStrings`, `TestHeaderCaptureLiteralAndNoCookieStoreByBinary`; pares `strings`, `boolean`, `null`, `mixed`, `literal` e `literal_text`: string tipada, aspas/escapes e nenhuma recursão |
| GO-242 | `TestHeaderCaptureLiteralStrings`, `TestHeaderCaptureGatesAndMixedBatch`, query `TestHeader`; pares `repeated`, `repeated_empty`, `comma`: primeira ocorrência, caixa, vazio e vírgula |
| GO-243 | `TestHeaderCaptureGatesAndMixedBatch`, `TestHeaderCaptureAtomicPublicationAndConflicts`, CLI `TestHeaderCaptureCLI`; pares `empty`, `empty_text`, `missing` |
| GO-244 | `TestHeaderCaptureGatesAndMixedBatch`, `TestHeaderCaptureFailuresNeverPublish`; pares `status_first`, `assertion_first`: primeira falha, erro JSON, curinga e sem expectativa |
| GO-245 | `TestMixedHeaderCapturePublishesTypedValues`, `TestHeaderCaptureGatesAndMixedBatch`, `TestHeaderCaptureAtomicPublicationAndConflicts`; par `mixed`: publicação tipada ou lote inteiro rejeitado |
| GO-246 | `TestHeaderCaptureAtomicPublicationAndConflicts`, `TestHeaderCaptureFailuresNeverPublish` e regressões de colisão de `Session.Publish`/US-06; par `collision`: valores anteriores preservados |
| GO-247 | `TestHeaderCaptureLiteralAndPositions`, `TestHeaderCaptureRejectsWholeFile`, `TestHeaderCaptureLateSyntaxPreventsAllRequests`, `TestCaptureStructuralCorpusByBinary`; novos negativos e pares `invalid_late`, `filter`, LF/CRLF |
| GO-248 | `TestSensitiveHeaderCaptureProvenanceAndPendingProtection`, `TestHeaderCaptureCLI`, `TestHeaderCaptureWritersAndDerivedProtection`, `TestSensitiveHeaderCaptureLaterFailureByBinary`: quatro nomes sensíveis, composição/escapes, marcas explícitas, lote rejeitado e operandos pendentes |
| GO-249 | `TestHeaderCaptureLiteralAndNoCookieStoreByBinary`, `TestHeaderCapturesUseOnlyCompleteFinalResponse`, `TestHeaderCaptureFinalAndIncompleteByBinary`: sem Cookie implícito, valor Set-Cookie só em contexto explicitamente declarado |
| GO-250 | 18 novos pares diferenciais e corpus completo, `TestHeaderCaptureHelpAndPublishedExample`, cache versionado e documentação atualizada; diferenças de tipagem, repetição, ordem e colisões registradas em `compatibility.md` |
| GO-251 | `TestHeaderCapturesUseOnlyCompleteFinalResponse`, `TestHeaderOnlyCaptureConsumesClosesAndDiscardsBody`, `TestHeaderCaptureFinalAndIncompleteByBinary`: somente final completa, body não retido e consumo/close antes de callback |
| GO-252 | Mesmos testes de transporte, modos read/close/timeout/cancel/incomplete; `TestHeaderCaptureFailuresNeverPublish`: sem publicação/resposta concluída/envio extra/retry |
| GO-253 | `TestHeaderCaptureWritersAndDerivedProtection`, `TestHeaderCaptureReportFailurePreservesCounts`: write/short write após sucesso, divergência e captura falha, 2 prioritário e writer não reutilizado |
| GO-254 | `TestDynamicSecretDeclarationsAreSessionLocal`, `TestHeaderCaptureReusedRunnerStartsClean`, callbacks das tabelas de segurança: uso antecipado, sessão nova, cenário/contexto imutáveis e sem coleções operacionais |


## US-21 — issue #40 / história 0021

Plano/arquitetura/tasks da issue #40 executados sem planner, somente backend,
na ordem da fila 0021: `back:1` no commit `5ebb1da` (fatia textual completa),
`back:2` no commit `94f857a` (segurança/recursos) e `back:3` nesta alteração
(corpus, ajuda, documentação e gates). Sem frontend/estáticos, fonte Hurl,
mudança de transporte de produção, versão Go ou configuração de CI/lint.
A dependência nova autorizada pelo plano é `golang.org/x/text v0.44.0`, isolada
em `query`; `go mod tidy` executado. YAML `v3.0.6` e módulo preservados.
SonarQube não executado: é exclusivo do QA. Evidências locais não substituem
aprovação pelo gate de revisão.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`. Ferramentas existentes `goimports` e
`golangci-lint` expostas por `/home/ff/go/bin` no PATH. Hurl local era 8.0.1;
após autorização explícita do usuário, foi baixado somente o binário oficial
**Hurl 8.0.0** (release Orange-OpenSource/hurl), extraído em
`/tmp/httpgo-0021-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` e anteposto ao PATH
apenas para o diferencial. SHA-256 conferido com o publicado:
`6ab1d6703b94086ece6d2f994d73f8d215926997b292ba29b894799425afbf67`.
Nenhuma instalação global/substituição do Hurl local; aviso de metadados libcurl
não impediu a execução. Não foi consultado código-fonte Hurl.

- Task 1: `go test ./internal/parser ./internal/query ./internal/assertion
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` passaram antes do commit.
- Task 2: `go test ./internal/runner ./internal/cli ./internal/transport` e
  `go test ./tests/e2e -count=1` passaram; `make fmt lint test`, `go vet ./...` e
  `go build ./...` também passaram antes do commit. Transporte de produção intacto.
- Task 3: novos pares executados com
  `go test -tags compatibility ./tests/e2e -run '^TestCompatibility$/^body_' -count=1 -v`;
  os **19 pares** passaram sem atualizar goldens. `TestCompatibilityBodyFixture`
  também passou, comprovando bytes inválidos, vazio explícito, ausência/JSON legado
  e preservação do Content-Type/charset configurado.
- `make compatibility` passou com **158 pares**, Hurl **8.0.0**, sem skip e sem
  regenerar as **139 observações anteriores**. Inclusão intencional dos 19 novos
  pares em `testdata/compatibility/` amplia o contrato publicado.
- A revisão local da conversão identificou uma fronteira de RN-02: reencodificar
  todo o body para validar U+FFFD rejeitaria bytes GB18030 válidos não canônicos
  próximos ao caractere. `query` passou a verificar somente a sequência original
  de U+FFFD. Regressões positivas/negativas e par `body_gb18030_replacement`
  passaram contra Hurl 8.0.0, sem afrouxar rejeição de bytes inválidos.
- Gate integrado: `make fmt lint test`, `go vet ./...`, `go build ./...` e
  `make e2e` passaram. Lint **0 issues**; testes com **race e cobertura**.
  Cobertura observada: assertion **92,7%**, parser **96,2%**, query **95,6%**,
  runner **93,2%**, CLI **97,1%**, transport **90,4%**, suite **95,4%**.
  E2E usa build único do binário, servidor HTTP local em porta dinâmica,
  stdout/stderr e exit codes; não depende de dummy/8080.
- Fuzzing: seeds do parser ampliados com body válido/inválido, LF/CRLF e ordem
  mista. Passaram os três comandos com `-fuzztime=20s -parallel=2`:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$'` (**53.590 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'` (**71.412 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$'` (**212.495 execuções**).
  Sem crash/corpus de falha. As contagens descrevem apenas esta execução.
- `gofmt -l` dos arquivos Go alterados e `git diff --check` sem apontamentos.
  Links locais novos e fixtures referenciados conferidos.
- Não executados builds cruzados ou testes nativos macOS/Windows: opcionais nesta
  história conforme plano v4, sem anunciar evidência inexistente.

### Rastreabilidade dos cenários

| GO | Evidência efetivamente executada |
|---|---|
| GO-226 | `TestBodyAssertionOrder`, `TestBodyCLI`, `TestBodyByBinary`, `TestBodyPublishedDemo`: demonstração, ambos os textos proibidos, códigos e próximo envio |
| GO-227 | `TestBodyLiteralSubstring`, `TestBodyExpectedNeverRenders`, `TestBodyByBinary`, par `body_json_original`: texto não JSON, escapes/whitespace originais e placeholders literais definidos/ausentes |
| GO-228 | Tabelas `TestBodyLiteralSubstring` e `TestBodyByBinary`; pares `body_case`, `body_spaces`, `body_newline`, `body_unicode_form` |
| GO-229 | Mesmas tabelas e quatro pares `body_empty_*`: todas as combinações de body/esperado vazios e não vazios |
| GO-230 | `TestBodyText`, `TestBodyByBinary`, pares UTF-8 omitido/declarado, ISO-8859-1 e UTF-16LE, incluindo U+FFFD válido |
| GO-231 | `TestBodyText`, `TestBodyAssertionOrder`, `TestBodyCLI`, `TestBodyByBinary`: charset desconhecido e sequências inválidas/truncadas, negação/esperado vazio não aprovados |
| GO-232 | `TestBodyAssertions`, `TestInvalidBodyAssertions`, `TestBodyInvalidLateByBinary`: literal obrigatório, operando extra, predicado fora do recorte, sem HTTP e zero contatos; LF/CRLF/indentação |
| GO-233 | `TestBodyAssertionOrder`: status, header, body e JSON intercalados; primeiro erro/falha prevalece, HTTP *, capturas só após sucesso |
| GO-234 | `TestBodyReportsNeverRetainOperationalData`, `TestPendingSecretConcealsAssertionsAndMetadata`, `TestBodySecretsAndPendingCapturesCLI`, `TestBodySecretExpectedByBinary`: conhecidos/derivados/pendentes, captura que falharia não antecipada, callbacks/coleção/streams sem body ou segredos |
| GO-235 | 19 pares `body_*`, corpus legado completo, `TestBodyHelp`, `TestBodyPublishedDemo`, ajuda/README/subset/arquitetura/matriz e fuzzing |
| GO-236 | `TestBodyFinalResponseAndConsumptionCancellation`, `TestBodyFinalResponseAndIncompleteByBinary` e retenção de headers já coberta em `TestHeaderPrecedenceAndPublication`: somente charset/texto finais, body exclusivo retido, header isolado sem retenção |
| GO-237 | `TestBodyAssertionsRequireReadAndClose`, `TestBodyFinalResponseAndConsumptionCancellation`, `TestBodyFinalResponseAndIncompleteByBinary`, `TestBodyConsumptionRealCancellation`: leitura/fechamento, prazo, cancelamento real durante consumo, sem aprovação parcial ou próximos envios |
| GO-238 | `TestBodyCallbackFailurePreservesResultsAndStops`, `TestBodyWriterFailureOverridesAllOutcomes`, `TestBodyRealOutputFailure`: aprovação/divergência/erro textual, callback/erro/short write, prioridade 2, sem repetir writer nem avançar |
| GO-239 | `TestBodyReportsNeverRetainOperationalData`, `TestBodyScenarioAndSessionsAreIndependent`, `TestBodyAssertionOrder`: cenário imutável, remoção de literais/bytes operacionais antes dos relatórios, sessões novas, capturas após todas as assertions aprovadas |


## US-16 — issue #38 / história 0016

Plano/arquitetura/tasks da issue #38 executados sem planner, somente backend.
Quatro tasks registradas e autoaprovadas na fila 0016, em ordem:
`back:1` no commit `5207fc4` (descoberta/carga/CLI), `back:2` no commit `c085537`
(agregação e interrupções), `back:3` no commit `98344ed` (isolamento/sigilo) e
`back:4` nesta alteração (contrato, fixtures e regressão integrada). Sem mudanças
de frontend/estáticos, dependências, toolchain, CI ou corpus diferencial.
SonarQube não executado: exclusivo do QA. Evidências locais não substituem
aprovação pelo gate automático de revisão.

### Ambiente e comandos efetivamente executados

Linux amd64, usuário não privilegiado (uid 1000), Go `go1.26.10-X:nodwarf5`, módulo
`capimgrosso.com/httpgo`, YAML `go.yaml.in/yaml/v3 v3.0.6` mantidos. Ferramentas já
existentes `goimports` e `golangci-lint` em `/home/ff/go/bin`, acrescentado ao PATH.
Hurl **8.0.0** já existente em
`/tmp/httpgo-0014-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` foi anteposto ao PATH
somente para o diferencial; sem instalar ferramentas ou ler fontes Hurl. O aviso
local de metadados libcurl não impediu a execução.

- Task 1: `go test ./internal/suite ./internal/cli` e
  `go test ./tests/e2e -count=1` passaram. Além das falhas injetadas por instância,
  `TestSuiteDeniedAccessUnprivilegedProcess` comprovou permissão negada em processo
  Linux realmente sem privilégios (arquivo/diretório), zero envios com entrada
  válida misturada, e rejeição de FIFO sem abertura bloqueante. O teste prevê
  redução de credenciais caso seja executado como root; nesta evidência o filho
  herdou uid 1000. Bits chmod isolados sob root não foram tratados como evidência.
- Task 2: `go test ./internal/suite ./internal/cli ./internal/runner` e
  `go test ./tests/e2e -count=1` passaram, com relógio, executor e writers falsos,
  e binário real para códigos, Ctrl+C e `/dev/full` em stdout/stderr.
- Task 3: `go test ./internal/template ./internal/cli ./internal/runner
  ./internal/suite` e `go test ./tests/e2e -count=1` passaram. JSON externo opt-in,
  multipart, segredo inicial de processo, capturas e Set-Cookie foram exercitados
  no servidor real; bytes/headers operacionais conferidos sem sanitização indevida.
- Em cada task de código: `make fmt lint test`, `go vet ./...` e `go build ./...`
  passaram antes do commit. Lint **0 issues**, testes com **race e cobertura**.
  A primeira execução de lint da task 3 apontou três seletores redundantes no
  wrapper de sessão; corrigidos sem desabilitar checks.
- Task 4 / gate integrado: `go test ./internal/cli ./tests/e2e`,
  `make fmt lint test`, `go vet ./...`, `go build ./...`, `make build` e `make e2e`
  passaram. Cobertura final observada: suite **95,4%**, CLI **96,9%**, template
  **96,1%**, runner **93,0%**. E2E usa binário compilado pelo harness, servidor
  local em portas dinâmicas, streams e exit codes; não depende do dummy/8080.
- O primeiro gate integrado revelou bloqueio do teste PTY antigo: ele aguardava
  o processo antes de drenar a saída e a ajuda ampliada passou a preencher o
  buffer. `TestPlainOutputOnRealTTY` foi corrigido para leitura concorrente com
  término coordenado; nenhuma redução de verificação, retry ou aumento de prazo.
  Os gates completos passaram após a correção, com e sem `NO_COLOR`.
- `make compatibility` passou com Hurl **8.0.0**, **139 pares**, sem skip ou
  regeneração de observações. Nenhum arquivo em `testdata/compatibility/` mudou.
  Suíte/configuração/descoberta próprias são testadas fora desse corpus.
- Fuzzing manteve os três comandos exigidos, todos com sucesso:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**88.791 execuções**);
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**23.249 execuções**);
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**33.389 execuções**). Sem crash ou novo corpus de falha.
- `gofmt -l` nos arquivos Go alterados e `git diff --check` limpos.
  Não foi executada validação nativa macOS/Windows nem fallback entre unidades
  Windows; ficam para release, conforme plano v4, sem bloqueio desta história.

### Rastreabilidade dos cenários

| GO | Evidência aplicável em Linux |
|---|---|
| GO-210 | `TestSuiteDemoAndFailurePrecedence` usa os três fixtures `testdata/suites/basic/` e ambiente `local`; execução completa/fail-fast, paths, contagens e códigos; `TestSuitePolicyAndIncrementalPresentation` confere resumo anterior antes do próximo envio |
| GO-211 | `TestSuiteContinuationCountsAndClock`, `TestSuitePolicyAndIncrementalPresentation`, `TestSuiteDemoAndFailurePrecedence`: assertion, transporte e pré-validação com/sem fail-fast; restantes não executados, sem contagem inventada |
| GO-212 | `TestDiscoverNormalizedUniqueOrder`, `TestSuiteDiscoveryAndConfiguration`, `TestSuiteDiscoveryRealHTTP`: misturas, sobreposição, argumentos invertidos, absolutos/relativos, `.`/`..`, arquivo explícito sem extensão e auxiliares ignorados |
| GO-213 | `TestDiscoverLinks` (destino, ancestral, ciclo, quebrado e `link/../arquivo`) e caso link da CLI: descobertos ignorados; explícitos rejeitados antes de HTTP |
| GO-214 | `TestDiscoverErrorsDiscardWholePlan`, `TestSuiteDiscoveryAndConfiguration`, `TestSuiteDeniedAccessUnprivilegedProcess`: inexistência, vazios, não regular/FIFO, acesso negado e falhas injetadas; nenhum envio, sobreposição válida não vira vazio |
| GO-215 | `TestSuiteDiscoveryAndConfiguration` cobre configuração, ambiente e referência efetiva inválidos; `TestSuiteInitialOptionsAndPayloadOriginsRemainIndependent` confere opções/timeout/variáveis iguais; regressão config/process/environment completa |
| GO-216 | `TestSuiteErrorPrecedenceAndPerFileValidation`, `TestSuitePolicyAndIncrementalPresentation`, `TestSuiteDemoAndFailurePrecedence`: erro estrutural tardio impede prefixo do arquivo sem desfazer anteriores; continuação conforme política |
| GO-217 | `TestSuiteRealIsolationProtectionAndPayloadOrigins`, `TestSuiteInitialOptionsAndPayloadOriginsRemainIndependent`: captura não resolve no próximo arquivo, Cookie ausente em todas as requisições, variáveis iniciais preservadas, JSON externo/multipart relativos ao cenário com configuração externa |
| GO-218 | `TestSuiteContinuationCountsAndClock`, `TestSuiteStatusValidationDoesNotImplyRequestSuccess`, `TestSuiteErrorPrecedenceAndPerFileValidation`, `TestSuiteProtectedErrorCauseAndNonnegativeDuration`: estados/contagens, erro pré-envio, sem validação, captura após status aprovado, 2 > 1 e relógio determinístico não negativo |
| GO-219 | `TestSuiteProtectionSurvivesSessionChangesBeforeCollection`, `TestSuiteSecretsOnStreamsAndLateErrors`, `TestSuiteRealIsolationProtectionAndPayloadOrigins`: variantes/derivados em nome, assertion, parsing e resumo depois da troca de sessão; callbacks e coleção protegidos, operação intacta |
| GO-220 | `TestLoadFailureNeverExecutesPartialScenario` e `TestDiscoveryCancellationAndLateLoadFailure`: abrir/Stat/ler/fechar falham por instância; recurso fechado, nenhum cenário parcial, causa inspecionável sem texto bruto, anteriores preservados e próximo autorizado |
| GO-221 | `TestDiscoveryCancellationAndLateLoadFailure`, `TestSuiteCancellationAndPresentationAreGlobal`, `TestSuiteRealCancellationStopsLaterFiles`: descoberta, request, intervalo entre arquivos e Ctrl+C real; timeout isolado permite próximo sem fail-fast |
| GO-222 | `TestSuitePresentationFailureStopsAllFiles`, `TestSuiteLateSecretPresentationFailure`, `TestSuiteRealOutputFailureStopsNextFile`: erro e short write em entrada, resumo por arquivo, diagnóstico/parsing e agregado; não repete writer, não envia próximos, proteção e código 2 |
| GO-223 | `TestSuiteInvocationReuseHasNoStateOrOptionLeak`, `TestProtectionValuesDetachedFromOperationalContext`: reutiliza comando após suíte, execução única com opções default, capturas/proteções/resultados novos; snapshot não publica valores e não cria alias |
| GO-224 | `TestDiscoverEquivalenceUsesFilesystem`, `TestDiscoverNormalizedUniqueOrder`: paths externos relativos ao cwd, grafias de caixa conforme identidade real e hard links; unidades distintas/execução nativa Windows explicitamente não validadas nesta etapa |
| GO-225 | `TestSuiteHelpAndArguments`, testes de ajuda/versão/uso, `TestPlainOutputOnRealTTY`, regressão CLI/E2E e `TestCompatibility`; ajuda/README/subset/arquitetura/matriz alinhados, sem anunciar JUnit/paralelismo/cookie jar |


## US-15 — issue #36 / história 0015

Plano da issue #36 e arquitetura/tarefas executados sem planner, somente backend.
Três tasks registradas/autoaprovadas e concluídas em ordem: `back:1` no commit
`418a95d` (parser, avaliação, runner, CLI e E2E), `back:2` no commit `0d45403`
(proteção de regex/padrão em assertions com captura secreta pendente) e `back:3`
(documentação, ajuda, seeds de fuzz e matriz). Sem alterações de frontend/estáticos,
dependências, toolchain, transporte ou CI. SonarQube não executado: exclusivo do QA.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports` e `golangci-lint` existentes
em `/home/ff/go/bin` foram expostos via PATH. Hurl do PATH padrão era 8.0.1;
foi selecionado o binário Hurl 8.0.0 já existente em
`/tmp/httpgo-0014-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, sem substituir
instalação nem instalar dependências. E2E usa build existente do harness, binário
real, `httptest.Server` e portas dinâmicas.

- Task 1: `go test ./internal/parser ./internal/assertion ./internal/runner
  ./internal/cli` e `go test ./tests/e2e -run '^TestJSON' -count=1` passaram.
- Task 2: `go test ./internal/runner ./internal/cli ./internal/transport` e
  `go test ./tests/e2e -run '^TestJSON' -count=1` passaram.
- Task 3 / gate final: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build` e `make e2e` passaram. `make fmt` passou após acrescentar
  `/home/ff/go/bin` ao PATH; a primeira tentativa sem esse PATH falhou porque
  `goimports` não estava localizado, sem alterar código. Lint reportou **0 issues**;
  `make test` executou `go test ./... -race -cover` sem falhas. Os testes
  `go test ./internal/cli ./internal/parser ./tests/e2e` passaram após mudanças de
  ajuda/seeds.
- `make compatibility` passou com Hurl **8.0.0**, sem skip: corpus de **139 pares**,
  incluindo três novos `matches_*`. `^Pizza` e `Pizza` sem âncoras passaram em
  ambas ferramentas nos pares verificados; `^Pizza$` divergiu sobre `Mini Pizza`
  em ambas, com códigos HTTPGo 1/Hurl 4. O restante do corpus anterior permaneceu
  sem alterações.
- Fuzzing com `-fuzztime=20s -parallel=2` passou sem crash: parser `FuzzParse`
  (73.098 execuções), template `FuzzRender` (48.172) e `FuzzRenderJSON`
  (10.364). Nenhum corpus de crash foi adicionado.
- `gofmt -l` nos Go alterados e `git diff --check` limpos. macOS/Windows não
  foram executados; são opcionais nesta etapa pelo plano v4. Não se declara
  execução nativa nessas plataformas.

### Rastreabilidade

| GO | Evidência |
|---|---|
| GO-194, GO-202 | `TestJSONMatchesStringSemantics`, `TestJSONMatchesByBinary`: trecho, âncoras, string vazia e interrupção da próxima requisição |
| GO-195, GO-196, GO-203, GO-204 | `TestJSONMatchesPatterns`, `TestInvalidJSONMatches`, seeds FuzzParse: literal JSON, escape e placeholder; opções e Unicode pela semântica regexp padrão |
| GO-197 | `TestInvalidJSONMatches`, `TestJSONMatchesByBinary`: lookaround inválido tardio e zero contatos HTTP |
| GO-198, GO-205 | casos absent/numeric em runner e validações JSON existentes; ausência/tipo resultam mismatch; decode/percurso mantêm JSONFailed |
| GO-199 | testes de ordem/fail-fast de runner e `TestJSONMatchesByBinary` |
| GO-200, GO-206 | extensão de `TestPendingSecretConcealsAssertionsAndMetadata` e `TestJSONSecretsByBinary`, incluindo regex/padrão nas superfícies examinadas |
| GO-207, GO-208, GO-209 | regressões existentes de resposta final, corpo incompleto/cancelamento e prioridade de writers; `matches` usa a mesma retenção/ordem JSON |
| GO-201 | `TestCompatibility`, três pares Hurl 8.0.0, ajuda root/test e documentação subset/README/architecture |

README, ajuda, `docs/subset.md`, arquitetura, matriz e este registro descrevem somente
padrões implementados/testados. `matches` permanece limitado a valores string de
JSON; interpolação, headers/body bruto, captura regex e JSONPath ampliado seguem
fora do suporte.

## US-14 — issue #34 / história 0014

Plano da issue #34 e arquitetura/tarefas em
`httpgo-docs/historias/US-14-validar-estrutura-e-limites.md` executados sem planner,
somente backend. Três tasks registradas/autoaprovadas e executadas em ordem:
`back:1` no commit `1243fd4` (presença/quantidade), `back:2` no commit `93e1081`
(limites/falhas seguras), `back:3` nesta alteração (contrato/corpus/regressão).
Sem frontend/estáticos, novas dependências, toolchain, transporte, CI, lint ou
budgets alterados. SonarQube não executado: exclusivo do QA. Evidências locais
não substituem aprovação do gate automático de revisão.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5` conforme go.mod, goimports e golangci-lint
já disponíveis em `/home/ff/go/bin` (acrescentado ao PATH dos comandos). A instalação
corrente de Hurl é **8.0.1**, diferente do contrato **8.0.0**. Com autorização
explícita do usuário, foi baixado o binário oficial da release 8.0.0 para
`/tmp/httpgo-0014-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, sem substituir a
instalação existente nem ler fontes Hurl. SHA-256 do tarball conferido contra a
release: `6ab1d6703b94086ece6d2f994d73f8d215926997b292ba29b894799425afbf67`.
Esse diretório foi anteposto ao PATH somente nos comandos diferenciais; todos
exigiram Hurl 8.0.0 real, sem skip ou alteração de gate. Aviso local de metadados
da libcurl não impediu a execução.

HTTP local com `httptest.Server`, portas dinâmicas, contatos contados, binário
compilado uma vez por suíte e `exec.CommandContext`; fakes pequenos em runner/CLI.
Sem dummy/8080, sleeps, novas goroutines de produção ou credenciais reais.

- Task 1: `go test ./internal/parser ./internal/query ./internal/assertion
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -run 'TestJSON' -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` passaram.
- Task 2: `go test ./internal/assertion ./internal/parser ./internal/runner
  ./internal/cli ./internal/transport`, E2E `TestJSON`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` passaram.
- Final: `go test ./internal/cli ./tests/e2e`, `make fmt lint test`,
  `go vet ./...`, `go build ./...`, `make build` e `make e2e` passaram.
  Regressão repetida sem cache: `go test ./... -race -cover -count=1` passou.
  Lint **0 issues**; cobertura: assertion **94,1%**, CLI **96,2%**, parser **96,2%**,
  query **98,6%**, runner **92,7%**, transporte **90,4%**. `gofmt -l` dos Go
  alterados e `git diff --check` limpos; links Markdown locais conferidos.
- `make compatibility` passou duas vezes com **Hurl 8.0.0**: **136 pares**,
  **20 novos `structure_*`**, **116 anteriores inalterados**. Nenhum `-update`
  indiscriminado ou flag própria passada ao Hurl. A primeira rodada mostrou que
  Hurl também aceita a quantidade enorme em `count_huge`; somente esse novo
  esperado foi corrigido. Presença de null/vazios/false/zero foi observada antes
  de anunciar compatibilidade. Diferenças de gramática, pré-validação de count
  negativo/fracionário e códigos estão em `compatibility.md`.
- Fuzzing com `-run '^$' -fuzztime=20s -parallel=2`, sem falhas:
  - `go test ./internal/parser -fuzz '^FuzzParse$'`: **124.380** execuções na
    última rodada; seeds ampliados com presença, count, quantidades enormes,
    comparações e esperados inválidos. O negativo count em captura também
    comprovou diagnóstico explícito de construção não suportada, sem execução.
  - `go test ./internal/template -fuzz '^FuzzRender$'`: **75.156** execuções;
  - `go test ./internal/template -fuzz '^FuzzRenderJSON$'`: **50.992** execuções.
- macOS/Windows não executados; opcionais nesta história pelo plano v4, seção 7.
  Não se declara execução nativa nessas plataformas.

### Rastreabilidade dos cenários

| Cenário | Evidência automatizada |
|---|---|
| GO-179 | `TestJSONStructureDemonstrationByBinary` — fixture versionada, três itens/total positivo/id null; remoção de id retorna 1 e interrompe |
| GO-180 | `TestJSONPresenceAndCount`, `TestJSONStructureByBinary` — raiz, null, vazios, false, zero, propriedade/índice ausentes |
| GO-181 | `TestJSONPresenceAndCount`, `TestJSONStructureByBinary` — cinco comparadores, array vazio/elementos null, quantidade esperada/recebida sem coleção |
| GO-182 | Mesmos testes — string/objeto/número/boolean/null e ausência falham com 1 sem contagem implícita |
| GO-183 | `TestExactNumericOrder`, `TestJSONNumericBoundariesByBinary` — quatro comparadores com positivos/negativos, limites estritos/inclusivos e decimais negativos |
| GO-184 | `TestOrderedJSONRequiresNumber`, `TestJSONNumericTypesAndEvaluationByBinary` — tipos incompatíveis e ausência, sem coerção |
| GO-185 | `TestInvalidJSONAssertions`, `TestJSONStructureInvalidNeverSends`, `TestJSONNumericInvalidNeverSends`, `TestJSONInvalidLateNeverSendsByBinary`, testes globais de fixtures — esperados/sintaxe/queries inválidos, HTTP ausente, count em captura, zero envios |
| GO-186 | `TestJSONStructureOrder`, `TestJSONLimitsOrderAndCaptures` — status/lista/capturas, decode tardio, primeira falha e callback antes do próximo envio; captura publicada só após sucesso |
| GO-187 | `TestJSONLimitsSafeReports`, `TestJSONLimitsSecretsByBinary` — consulta e operandos conhecidos/derivados sanitizados antes do callback e em ambos os streams |
| GO-188 | `TestExactNumericOrder`, `TestValidCount`, `TestJSONNumericTypesAndEvaluationByBinary` — além de float64/int, decimais próximos, zero com sinal e expoentes enormes sem expansão |
| GO-189 | `TestJSONNumericTypesAndEvaluationByBinary`, regressões `TestJSONEqualityAndEvaluationByBinary` — resposta vazia/malformada/adicional e percurso por null/tipo incompatível são 2, não 1 |
| GO-190 | `TestJSONStructurePendingSecret`, `TestJSONLimitsSafeReports`, `TestJSONLimitsPendingSecretsCLI`, `TestJSONLimitsSecretsByBinary` — operandos/contagem ocultos com captura secreta pendente inclusive inválida, sem antecipá-la |
| GO-191 | `TestJSONAssertionRequiresSuccessfulReadAndClose`, `TestJSONAssertionFinalBodyDeadlineAndCancellation`, `TestJSONLimitsFinalResponseByBinary` — novas assertions somente após resposta final consumida/fechada; read/close/incompleto/timeout/cancelamento impedem próxima requisição |
| GO-192 | `TestJSONLimitsReporterPriorityAndImmutability`, `TestJSONLimitsWriterFailures` — erro/short write em stdout/stderr, prioridade 2, cenário intacto e ausência de documento/operandos brutos no relatório |
| GO-193 | `TestCompatibility`, `TestJSONAssertionHelp`, `TestJSONStructureDemonstrationByBinary` — Hurl 8.0.0, matriz/ajuda/exemplo e negativos versionados |

README, ajuda root/test, subset, arquitetura e matriz descrevem o suporte real.
Na entrega US-14, regex ainda estava fora do suporte; a US-15 adicionou somente
`matches` em strings selecionadas de JSON. Filtros, aritmética, agregações,
ampliação de JSONPath e count em capturas/strings/objetos continuam fora. Extensões
YAML/interpolação externa permanecem separadas.

## US-13 — issue #32 / história 013

Plano da issue #32 executado diretamente, sem planner e somente backend, após
rebase sobre a US-12 (`2678e1d`). Três tasks registradas/autoaprovadas na fila 013,
executadas na ordem: Task 1 `3184278` (referências efetivas); Task 2 `8c15319`
(proteção prévia, substitutos e superfícies); Task 3 nesta alteração (E2E, ajuda,
exemplos e evidências). Sem frontend/estáticos, novas dependências, alteração de
toolchain, CI, lint ou budgets. SonarQube não executado: responsabilidade exclusiva
do QA. Evidências locais não substituem o gate automático de revisão.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`, goimports, golangci-lint e Hurl **8.0.0**
disponíveis. Primeira tentativa de testes bloqueada pelo cache de módulos padrão
somente leitura; nova execução bem-sucedida com `GOMODCACHE=/tmp/httpgo-013-mod` e
`GOCACHE=/tmp/httpgo-013-cache`, sem modificar dependências ou instalar ferramentas.
E2E reutiliza build único, `exec.CommandContext`, servidores `httptest` em portas
dinâmicas, `Cmd.Env` explícito sem duplicatas para as variáveis testadas, distinção
entre ausência/vazio, canais/contextos e nenhum sleep. Valores fictícios fornecidos
no ambiente do filho, nunca credenciais reais nos argumentos/fixtures.

- Task 1: `go test ./internal/config ./internal/cli` passou; o E2E nominal passou
  na regressão completa. Expectativas antigas de rejeição global foram trocadas
  por composição parcial inválida, preservando validações YAML/ambientes.
- Task 2: `go test ./internal/config ./internal/template ./internal/runner ./internal/cli`
  passou. `make fmt lint test`, `go vet ./...` e `go build ./...` passaram antes de
  concluir cada task de implementação.
- Final: `go test ./internal/cli ./tests/e2e -count=1`, `make fmt lint test`,
  `go vet ./...`, `go build ./...`, `make build` e `make e2e` passaram.
  Lint **0 issues**; regressão com **-race -cover** sem falhas. Cobertura: config
  **96,0%**, CLI **95,2%**, runner **93,0%**, template **95,7%**, transporte **90,4%**.
  `gofmt -l` dos Go alterados e `git diff --check` limpos.
- `make compatibility` passou com **Hurl 8.0.0**, sem skip: **116 pares existentes**
  inalterados, sem regeneração de observações ou flags próprias fornecidas ao Hurl.
  YAML continua fora do corpus diferencial; aviso de metadados de versão da libcurl
  não impediu o gate.
- Fuzzing sem falhas, todos com `-run '^$'`, `-fuzztime=20s` e `-parallel=2`:
  - `go test ./internal/parser -fuzz '^FuzzParse$'`: **70.980** execuções;
  - `go test ./internal/template -fuzz '^FuzzRender$'`: **41.703** execuções;
  - `go test ./internal/template -fuzz '^FuzzRenderJSON$'`: **50.816** execuções.
- macOS/Windows não executados; opcionais nesta história pelo plano v4, seção 7.
  Não se declara execução nativa ou validação nessas plataformas.

### Rastreabilidade dos cenários da issue #32

| Cenário | Evidência automatizada |
|---|---|
| GO-161 | `TestProcessEnvironmentDemonstration`, `TestProcessEnvironmentRequest` — base e ambiente, header recebido por HTTP real, streams e código 0; fixture documentada executada |
| GO-162 | `TestProcessReferenceStrings`, `TestProcessEnvironmentJSONString`, `TestProcessEnvironmentPublicStringTypes` — strings numéricas/boolean/null, aspas, Unicode e escaping preservados |
| GO-163 | `TestProcessReferenceMissingAndCancellation`, `TestProcessEnvironmentInvalidBeforeNetwork`, `TestProcessEnvironmentPublicInvalidBeforeNetwork` — referência efetiva não usada ausente: 2, nome/chave/origem, zero HTTP |
| GO-164 | `TestProcessReferencePrecedence`, `TestProcessEnvironmentPublicPrecedence`, `TestProcessReferenceUnionProtectsEverySubstitute` — consultas contadas, fonte descartada ausente, overrides/união de marcas em falha e sucesso |
| GO-165 | `TestProcessReferenceValidationPerSource`, `TestProcessEnvironmentPublicInvalidBeforeNetwork` — gramática, nomes válidos, composição/múltiplas referências/delimitadores inválidos, rejeição por fonte com override |
| GO-166 | Mesmos testes de validação — referências em listas/objetos/listas internos rejeitadas antes de consulta/HTTP, mesmo substituídas |
| GO-167 | `TestProcessEnvironmentRequest`, `TestProcessEnvironmentPublicStringTypes` — processo e HTTPGO_* não referenciados não substituem URL, timeout ou dados |
| GO-168 | `TestProcessSecretIsSanitizedBeforeCallback`, `TestProcessEnvironmentEchoAssertions`, `TestProcessEnvironmentPublicEchoAssertionsAndEarlyFailures` — header comum/JSON, esperado ou recebido, comparação mantém divergência, callbacks/resultados e ambos os streams protegidos |
| GO-169 | `TestProcessEnvironmentProtectionAcrossBodies`, `TestProcessEnvironmentPublicMultipartAndExternal`, `TestProcessReferenceUnionProtectsEverySubstitute` — marcas automáticas/explícitas/CLI, capturas futuras, URL/header/inline/externo/multipart e arquivo literal |
| GO-170 | `TestProcessEnvironmentRequest`, `TestProcessReferenceStrings`, regressões `TestEnvironmentTypedComposition`, `TestEnvironmentStructuralConflicts`, testes YAML/timeout — tipos/irmãos/listas/null/prazo e conflitos preservados |
| GO-171 | Testes de strings em config/CLI/E2E — vazio presente enviado como string vazia, não ausência, sem substituição global de texto vazio |
| GO-172 | Mesmos testes de strings — `${OTHER}` e `{{other}}` chegam literalmente, sem expansão/consulta adicional |
| GO-173 | `TestProcessReferenceProtectionBeforeMissingDiagnostic`, `TestProcessEnvironmentEarlyProtection`, `TestProcessEnvironmentPublicEchoAssertionsAndEarlyFailures` — disponível coletado antes de informar ausente, nas duas ordens, inclusive em caminho de diagnóstico |
| GO-174 | `TestProcessEnvironmentEarlyProtection`, `TestProcessReferenceProtectionOnMergeConflict`, `TestProcessEnvironmentPublicEchoAssertionsAndEarlyFailures` — argumento/composição/parsing/arquivo ausente, segredo conhecido e substitutos protegidos antes da primeira sessão; conflito de merge não consulta referência descartada |
| GO-175 | `TestProcessReferenceLookupIsolation`, `TestProcessEnvironmentReuseIsolation` — mesmo comando, ambiente alterado e depois sem referências, sem cache de valores/marcas/capturas |
| GO-176 | `TestProcessReferenceMissingAndCancellation`, `TestProcessEnvironmentInvalidBeforeNetwork`, `TestProcessEnvironmentOutputAndTransportFailures`, `TestProcessEnvironmentRealCancellation`; regressões de fechamento de config/runner — cancelamento durante preparação, antes de rede e SIGINT real, sem próxima requisição ou contexto parcial |
| GO-177 | `TestProcessEnvironmentOutputAndTransportFailures`, `TestProcessEnvironmentRealOutputFailure`, `TestProcessEnvironmentPublicEchoAssertionsAndEarlyFailures` — erro/short write de writers, /dev/full Linux e conexão encerrada: 2, sem retry/próximo envio nem vazamento |
| GO-178 | `TestProcessEnvironmentHelp`, `TestProcessEnvironmentDemonstration` — ajuda, README/subset/arquitetura/matriz, exemplos versionados positivos e negativos; extensão e limites publicados sem anunciar dotenv/stdin/JUnit |

As seções US-11/US-12 abaixo são registros históricos: suas rejeições globais de
`${NOME}` foram intencionalmente substituídas na US-13. Permanecem negativas as
referências parciais/aninhadas/malformadas; os demais contratos seguem válidos.

## US-12 — issue #29 / história 012

Plano da issue #29 executado diretamente, sem planner, somente backend. Três
Tasks registradas, autoaprovadas e executadas na ordem da fila 012. Task 1:
`b488c82` (seleção/carga pela CLI); Task 2: `8a14253` (composição tipada,
proteção e isolamento); Task 3 nesta alteração (integração, ajuda, exemplos e
validação). Nenhum frontend/estático, dependência, toolchain, CI, regra de lint
ou budget alterado. SonarQube não executado (exclusivo do QA). Evidências locais
não substituem o gate automático de revisão da história.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`, goimports, golangci-lint e Hurl **8.0.0**
já disponíveis. O cache padrão de módulos é somente leitura e a primeira tentativa
de testes/lint falhou ao buscar YAML v3.0.6. Reexecução bem-sucedida com
`GOMODCACHE=/tmp/httpgo-012-gomod`, sem mudar go.mod/go.sum ou instalar ferramentas.
Servidores `httptest` em portas dinâmicas, build único do binário E2E,
`exec.CommandContext`, cwd somente no processo filho, canais/contextos e timers,
sem sleeps ou dummy. macOS/Windows não executados: opcionais pelo plano v4,
seção 7; nenhuma alegação de validação nessas plataformas.

- Task 1: `go test ./internal/config ./internal/cli` e
  `go test ./tests/e2e -run '^TestEnvironment' -count=1` passaram.
- Task 2: `go test ./internal/config ./internal/template ./internal/cli ./internal/runner`
  e `go test ./internal/transport` passaram.
- `make fmt lint test`, `go vet ./...` e `make build` passaram antes de concluir
  cada task. Validação integrada: `go test ./internal/cli ./tests/e2e`,
  `make fmt lint test`, `go vet ./...`, `go build ./...`, `make build` e
  `make e2e` passaram; lint **0 issues**, testes **-race -cover** sem falhas.
  Cobertura: config **94,8%**, CLI **95,2%**, runner **93,0%**, template **95,7%**,
  transporte **90,4%**. `gofmt -l` dos Go alterados e `git diff --check` limpos.
- `make compatibility` passou com Hurl **8.0.0**, sem skip: **116 pares** existentes,
  sem alterações em `testdata/compatibility/`, regeneração de observações ou flags
  próprias fornecidas ao Hurl. Aviso local de versão da libcurl não impediu o gate.
- Fuzzing sem falhas:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    **40.244** execuções.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    **103.301** execuções.

### Rastreabilidade dos cenários da issue #29

| Cenário | Evidência automatizada |
|---|---|
| GO-141 | `TestEnvironmentDemonstration`, `TestEnvironmentDocumentedFixtures` — binário real, dois servidores, Pizza/4500 e Suco/1200, active herdado, status 201 |
| GO-142 | `TestEnvironmentTypedComposition`, `TestEnvironmentBodiesAndCaptures`, `TestEnvironmentDocumentedFixtures` — CLI prevalece, irmãos/tipos/precisão e objetos opacos |
| GO-143 | `TestEnvironmentTypedComposition` — escalares, listas e null substituem integralmente folhas, string numérica mantém tipo |
| GO-144 | `TestEnvironmentStructuralConflicts`, `TestEnvironmentConflictsCannotBeHiddenByCLI`, `TestEnvironmentInvalidDocumentsNeverSend` — conflitos em fontes/merge, mapas vazios e duplicação CLI não escondidos por override, zero envios |
| GO-145 | `TestEnvironmentSelection`, `TestEnvironmentInvalidBeforeHTTP`, `TestEnvironmentInvalidDocumentsNeverSend` — nomes ASCII permitidos, inválidos/traversal, vazio, falta de argumento e repetição idêntica/diferente |
| GO-146 | `TestEnvironmentRoots`, `TestEnvironmentSelection`, `TestEnvironmentEmptyLimitAndNoAncestor` — base externa inclusive vazia fixa raiz, sem base usa cwd, não cenário/ancestral |
| GO-147 | `TestEnvironmentInvalidBeforeHTTP`, `TestEnvironmentInvalidDocumentsNeverSend` — arquivos inválidos não selecionados não interferem |
| GO-148 | `TestEnvironmentDocuments`, `TestEnvironmentInvalidDocumentsNeverSend`, `TestEnvironmentDemonstration` — versão opcional/1, rejeições YAML/schema/nomes/tipos/recursos antes da rede |
| GO-149 | `TestEnvironmentInvalidBeforeHTTP`, `TestEnvironmentInvalidDocumentsNeverSend`, `TestEnvironmentDocumentedFixtures` — referência rejeitada com processo definido e override CLI |
| GO-150 | `TestEnvironmentProtectionAllSources`, `TestEnvironmentTimeoutsAndEmptyOverrides`, `TestEnvironmentSecretAssertions`, `TestEnvironmentReuseAndCaptureIsolation` — união de marcas, valores anteriores/substitutos/variantes, lista vazia, captura futura e marca sem valor |
| GO-151 | `TestEnvironmentReadFailuresAndLimit`, `TestEnvironmentRoots`, `TestEnvironmentInvalidDocumentsNeverSend` — inexistente/ilegível, causa segura e nenhum fallback/envio |
| GO-152 | `TestEnvironmentDocuments`, `TestEnvironmentTimeoutsAndEmptyOverrides`, `TestEnvironmentEmptyLimitAndNoAncestor` — vazio/whitespace/comentários conserva base/CLI/defaults |
| GO-153 | `TestEnvironmentReadFailuresAndLimit`, `TestEnvironmentEmptyLimitAndNoAncestor`, `TestEnvironmentInvalidDocumentsNeverSend` — 1 MiB aceito, byte extra rejeitado durante leitura, inclusive só comentários |
| GO-154 | `TestEnvironmentTimeoutsAndEmptyOverrides`, `TestEnvironmentTimeoutMetadata`, `TestEnvironmentPublicTimeout` — herança/extremos/precisão/faixa, orçamento em redirects/consumo e próxima entrada impedida; regressões US-11 de prazo maior que dez segundos mantidas |
| GO-155 | `TestEnvironmentReadFailuresAndLimit`, regressões `TestResourceFailures` e cancelamento do runner — funções privadas de I/O, leitura/fechamento com falha, contexto, descritor fechado e nenhum conteúdo parcial aplicado |
| GO-156 | `TestEnvironmentProtectionBeforeSession`, `TestEnvironmentProtectionAllSources`, `TestEnvironmentInvalidDocumentsNeverSend` — erro de composição/CLI/cenário com marca só no ambiente protege valores conhecidos antes da sessão; YAML sem conteúdo bruto |
| GO-157 | `TestEnvironmentReuseAndCaptureIsolation` — mesmo comando/executor com local/staging/local/sem seleção, timeout e marcas isolados, captura indisponível na próxima invocação |
| GO-158 | `TestEnvironmentBodiesAndCaptures` — captura → inline → JSON externo opt-in → multipart, base/cwd externos, payloads relativos ao cenário e bytes multipart literais |
| GO-159 | `TestEnvironmentWriterFailureKeepsPriorityAndProtection`, `TestEnvironmentProcessOutputFailure` — erro/short write e /dev/full real Linux em stdout/stderr, prioridade 2 após divergência, sem próxima entrada, resumo reimpresso ou exposição |
| GO-160 | `TestEnvironmentHelp`, `TestEnvironmentDocumentedFixtures`, regressões `TestHelpAndBuildMetadata` e `TestPlainOutputOnRealTTY` — extensão, seleção única, raiz, precedência, limites e distinção do ambiente do processo, flags/versão/streams/TTY |

Exemplos YAML de ambientes positivos/negativos em `testdata/config/environments/`,
base em `testdata/config/environment_base.yaml` e cenário em
`testdata/scenarios/scenario_environment_products.hurl`. São separados do corpus
Hurl. README, subset, arquitetura, ajuda e matriz refletem o suporte implementado:
sem fontes do processo, stdin, composição de vários ambientes ou mudança de
resolução de payloads.

## US-11 — issue #27 / história 011

Plano da issue #27 executado diretamente, sem planner, somente backend. Quatro
Tasks registradas e autoaprovadas na fila 011, implementadas sequencialmente.
Tasks 1–3: `37dfbb3` (carga YAML segura/CLI), `5d928fc` (composição/proteção) e
`e930687` (timeout por requisição). Task 4 nesta alteração: coexistência, precisão
além da faixa de máquina, exemplos, ajuda e documentação/evidências integradas.
Nenhuma alteração em frontend/estáticos, toolchain, CI, regras de lint ou budgets.
SonarQube não foi executado; é responsabilidade exclusiva do QA. Estes resultados
locais não substituem o gate de revisão da história.

### Ambiente, dependência e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`, goimports, golangci-lint e Hurl **8.0.0**.
A biblioteca YAML prevista pelo plano foi fixada em `go.yaml.in/yaml/v3 v3.0.6`,
release de 2026-10-13 do repositório mantido pela organização YAML
(`https://github.com/yaml/go-yaml`, metadados consultados com `go list -m -json`).
Uso isolado em `internal/config`; `go mod tidy` executado e go.mod/go.sum revisados.
Cache padrão de módulos/sumdb somente leitura: usados `GOPATH=/tmp/httpgo-011-go`
e `GOMODCACHE=/tmp/httpgo-011-modcache`, sem mudar a versão do Go. A ferramenta
`govulncheck v1.9.0`, ausente inicialmente, foi instalada **com autorização do
usuário**, somente em `/tmp/httpgo-011-tools`, sem dependência do projeto.
`govulncheck ./...` concluiu: **No vulnerabilities found.**

HTTP/TLS local em portas dinâmicas, build único do binário E2E, CommandContext,
diretórios temporários e cwd alterado somente no filho. Sem dummy, porta fixa,
sleeps ou dependências de runtime. O teste público de prazo maior espera timer
de 10,1 segundos sob limite de 15 segundos; o teste público anterior de timeout
default de dez segundos permanece inalterado. Testes nativos/cross-builds
macOS/Windows não realizados: opcionais nesta história conforme plano v4, seção 7.

- Task 1: `go test ./internal/config ./internal/cli` e
  `go test ./tests/e2e -run '^TestConfig' -count=1` passaram.
- Task 2: `go test ./internal/config ./internal/template ./internal/cli ./internal/runner`
  passou; seeds pertinentes do JSON renderer ampliados.
- Task 3: `go test ./internal/config ./internal/runner ./internal/transport ./internal/cli`
  e `go test ./tests/e2e -run '^TestConfig' -count=1` passaram.
- `make fmt lint test` passou antes de concluir cada task e na validação integrada:
  lint **0 issues**, `go test ./... -race -cover` sem falhas. Cobertura final:
  config **95,0%**, CLI **94,9%**, runner **93,0%**, template **95,7%**, transporte
  **90,4%**. Também passaram explicitamente `go vet ./...`, `go build ./...`,
  `go test ./internal/cli ./tests/e2e` e `make e2e`.
  `gofmt -l` dos Go alterados/novos e `git diff --check` sem pendências.
- `make compatibility` passou com Hurl **8.0.0**, sem skip: **116 pares**,
  `testdata/compatibility/` inalterado, sem regenerar observações ou fornecer
  `--config`/`--render-files` ao Hurl. O aviso local de versão da libcurl não
  impediu a identificação da versão nem o diferencial.
- Fuzzing executado separadamente, sem falhas:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    **23.760** execuções, passou.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    **20.760** execuções, passou.

### Rastreabilidade dos cenários da issue #27

| Cenários | Evidência automatizada |
|---|---|
| GO-119 | `TestConfigDiscoveryAndSelection`, `TestConfigTypedRequest`, `TestConfigDocumentedFixturesAndOverrides` — URL/body tipados efetivamente recebidos |
| GO-120 | `TestConfigDiscoveryAndSelection`, `TestConfigDocumentedFixturesAndOverrides` — explícito fora do cwd prevalece sobre padrão inválido, inexistente sem fallback |
| GO-121 | `TestTypedHierarchy`, `TestConfigHierarchyNotAnObject`, `TestPlainNumbersBeyondMachineRangeKeepType`, `TestConfigExternalOriginAndMixedModes` — hierarquia, strings numéricas, precisão aninhada, números grandes/pequenos sem float64 |
| GO-122 | `TestComposeExactOverrides`, `TestConfigDocumentedFixturesAndOverrides` — irmãos preservados, listas/null substituídos, objetos opacos, conflitos e duplicação CLI rejeitados |
| GO-123 | `TestConfiguredTimeoutOwnsWholeOperation`, `TestTimeoutMetadataIsolationAndCaller`, `TestConfigPublicFractionalTimeout` — orçamento único em redirects/body, interrupção e isolamento |
| GO-124 | `TestOptionalEmptyAndLimit`, `TestConfigEmptyAbsentAndNoAncestor` — ausência/vazio/comentários/defaults e nenhuma busca em ancestral |
| GO-125 | `TestRejectedDocuments`, `TestConfigInvalidBeforeHTTP`, `TestConfigInvalidDocumentsNeverSend` — rejeições localizadas, código 2, zero envios |
| GO-126 | `TestTimeoutExactMilliseconds`, `TestConfigTimeoutMetadataAndDiagnostic`, `TestConfigInvalidDocumentsNeverSend` — tipo/faixa/precisão sem coerção ou arredondamento |
| GO-127 | `TestProtectionUnionBeforeAndAfterCompose`, `TestConfigProtectionReachesCallback`, `TestConfigSecretProtectionByBinary`, `TestConfigExternalOriginAndMixedModes` — proteção base/CLI/derivados em callbacks e streams de sucesso/erro |
| GO-128 | `TestConfigProtectionCapturesAndIsolation`, `TestConfigMissingAndInitialCaptureConflict`, `TestConfigExternalOriginAndMixedModes` — marca sem valor, ausência falha, captura futura protegida e colisão com inicial não sobrescreve |
| GO-129 | `TestRejectedDocuments`, `TestConfigDocumentedFixturesAndOverrides` — `${NOME}` rejeitado inclusive com variável de processo definida e override CLI |
| GO-130 | `TestRejectedDocuments`, `TestConfigDocumentedFixturesAndOverrides` — âncora/alias/tag explícita, inclusive aninhada, sem expansão ou mensagem bruta |
| GO-131 | `TestOptionalEmptyAndLimit`, `TestConfigInvalidDocumentsNeverSend` — exatamente 1 MiB aceito, byte adicional rejeitado, também só comentários, leitura limitada sem Stat |
| GO-132 | `TestTimeoutExactMilliseconds`, `TestConfigTimeoutMetadataAndDiagnostic` — extremos 1 ms/24 h, omissão de timeout mantém dez segundos |
| GO-133 | `TestResourceFailures` — funções privadas de abertura, falhas de leitura/fechamento, causas preservadas e nenhum dado parcial aplicado |
| GO-134 | `TestConfigProtectionBeforeSession`, `TestConfigInvalidBeforeHTTP` — erros de CLI/contexto/cenário e nomes de arquivo protegidos antes da sessão; YAML nunca ecoado |
| GO-135 | `TestConfigProtectionCapturesAndIsolation`, `TestConfigCommandIsolationAcrossSettings`, `TestTimeoutMetadataIsolationAndCaller`, `TestTimeoutOptionIsPerRunMetadata` — mesmo comando/executor com configurações diferentes e depois defaults, sem vazamento |
| GO-136 | `TestResourceFailures`, `TestTimeoutMetadataIsolationAndCaller`, `TestConfigRealCancellation` — cancelamento antes da carga/envio, prazo menor do caller e Ctrl+C real durante body sob timeout YAML, sem próxima entrada |
| GO-137 | `TestConfigPublicTimeoutAboveTenSeconds`, `TestTimeoutMetadataIsolationAndCaller`, `TestConfigTimeoutMetadataAndDiagnostic` — processo real conclui após dez segundos, limites internos removidos, diagnóstico efetivo |
| GO-138 | `TestConfigHelpDocumentsOnlyImplementedSources` e regressões `TestHelpAndBuildMetadata`, `TestPlainOutputOnRealTTY` — ajuda, flags, versão, streams e TTY Linux |
| GO-139 | `TestConfigExternalOriginAndMixedModes` — captura → inline → JSON externo opt-in → multipart, configuração/cwd externos sem mudar origem dos payloads, bytes multipart intactos |
| GO-140 | `TestConfigWriterFailureKeepsPriorityAndProtection`, `TestConfigProcessOutputFailure` — erros/short writes em unitários e falha real `/dev/full` Linux em stdout/stderr; código 2, sem próxima entrada ou exposição |

Exemplos e negativos YAML ficam em `testdata/config/`, separados do corpus Hurl.
Ajuda, README, subset, arquitetura e matriz refletem somente o suporte real;
ambientes, HTTPGO_*, referências de processo, stdin e hooks continuam indisponíveis.
Não há alegação de compatibilidade Hurl geral ou validação nativa fora de Linux.

## US-10 — issue #25 / história 010

Plano da issue #25 executado diretamente, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 010 antes da implementação, executadas
sequencialmente. Tasks 1–3 nos commits `9d8aa73`, `835b737` e `060b905`;
documentação, exemplos, ajuda e validação integrada da task 4 nesta alteração.
Sem frontend/estáticos, dependências, toolchain, CI ou gates alterados. Não foi
executado SonarQube (responsabilidade QA). Evidência local não substitui review.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`, goimports, golangci-lint e Hurl **8.0.0**
já disponíveis, sem instalação. HTTP/TLS local em portas dinâmicas; build único
do binário nos E2E, `exec.CommandContext`, arquivos temporários e diretório do
processo filho, sem chdir global ou sleeps. Sem dependência do dummy/porta 8080.
Cross-builds e execução nativa macOS/Windows não realizados: opcionais para esta
história pelo plano v4, seção 7; não há alegação de validação nessas plataformas.

- Task 1: `go test ./internal/template ./internal/runner ./internal/cli` e
  `go test ./tests/e2e -run 'TestExternal' -count=1` passaram. Demonstração Pizza/45/true
  e Suco/12/false com a flag antes/depois do arquivo; invocação seguinte sem flag
  mantém placeholders literais. Valores completos/raiz, alta precisão, null,
  arrays/objetos, escalares em strings, escapes/Unicode e inserção não recursiva.
  JSON estático preserva whitespace externo LF/CRLF; nomes/caminhos literais,
  absolutos e relativos, sem seleção por extensão ou inferência de Content-Type.
- Task 2: `go test ./internal/template ./internal/parser ./internal/runner
  ./internal/cli` e `go test ./tests/e2e -run 'TestExternal' -count=1` passaram.
  Falhas na segunda entrada preservam a primeira resposta e suas contagens,
  sem enviar a afetada/seguintes. Vazio, whitespace, não JSON, UTF-8 inválido,
  múltiplos valores, instrução Hurl no payload, template malformado, variável
  ausente, objeto opaco, array/objeto em string e chave dinâmica são rejeitados.
  LF/CRLF e expansão anterior não deslocam a origem; renderer falso exercita a
  defesa de JSON final inválido com fallback 1:1. I/O mantém FileError, causas
  inspecionáveis e bytes parciais descartados; fakes cobrem leitura/fechamento.
  Descritor real fechado antes do envio; cancelamento no renderer impede executor.
- Task 3: `go test ./internal/runner ./internal/transport ./internal/cli -race`
  e `make e2e` passaram. Capturas anteriores de todos os tipos, inclusive raiz,
  alta precisão e texto semelhante a placeholder; callbacks síncronos sem body,
  headers, planos ou referência operacional. Segredos fictícios iniciais e
  capturados/derivados/escapes conhecidos não aparecem em callbacks ou streams
  de sucesso, divergência, renderização, transporte ou writer. Origem externa e
  referência do cenário também são sanitizadas. Writer conserva prioridade 2.
  Fluxo captura → inline → externo → multipart → GET com a flag preserva modos;
  arquivo multipart binário com placeholder indefinido permanece literal.
  Status/assertion divergente retorna 1; consumo incompleto/renderização retorna
  2 e impede próxima entrada. Redirects 301/302/303/307/308 HTTP e TLS local
  verificam método/bytes renderizados, sem mudanças de produção no adapter.
- Task 4: `go test ./internal/cli ./tests/e2e` passou. Ajuda root/test distingue
  modo bytes de extensão HTTPGo e multipart; exemplo versionado e negativos de
  chave dinâmica/template malformado são executados pelo binário, com posição,
  código e contatos verificados. Seeds de parser e JSON renderer ampliados.
- `make compatibility` passou com Hurl **8.0.0** obrigatório, sem skip:
  **116 pares anteriores inalterados**, sem qualquer alteração em
  `testdata/compatibility/`, sem `-update` ou flag própria passada ao Hurl.
  A extensão tem testes separados, não promete renderização equivalente.
  Hurl imprime aviso local de versionamento da libcurl ao iniciar, sem impedir
  identificação da versão ou os resultados do diferencial.
- Fuzzing executado separadamente, sem falhas encontradas:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    passou, **64.348** execuções.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    passou, **214.366** execuções.
- `make fmt lint test` passou antes de concluir cada task e na validação integrada:
  lint **0 issues**, `go test ./... -race -cover` sem falhas. Cobertura integrada:
  runner **93,0%**, CLI **95,0%**, template **95,7%**. Também passaram explicitamente
  `go vet ./...`, `go build ./...` e `make e2e`. `gofmt -l` dos Go alterados e
  `git diff --check` sem pendências. Nenhum gate removido/reduzido ou ferramenta
  instalada para contornar validação.

### Rastreabilidade dos cenários da issue

| Cenários | Evidência automatizada |
|---|---|
| GO-01 | `TestRenderFilesPerInvocation`, `TestExternalRenderedDemoByBinary`, `TestExternalRenderedDocumentedScenarios` |
| GO-02 | `TestExternalJSONValuesAndStaticBytes`, `TestExternalCapturedRootTypes`, `TestExternalRenderedMixedModesByBinary` |
| GO-03 | `TestExternalJSONValuesAndStaticBytes`, `TestExternalCapturedRootTypes`, `TestExternalRenderedRedirectsAndTLS` |
| GO-04–06 | `TestExternalRenderFailuresUseOriginalPayloadPositions`, `TestExternalDefensiveFinalJSONValidation`, `TestExternalRenderedFailuresByBinary` |
| GO-07 | `TestRenderFilesPayloadDiagnostic`, `TestExternalRenderFailuresUseOriginalPayloadPositions`, `TestExternalRenderedStructuralErrorPreventsAllNetwork` |
| GO-08 | `TestExternalBytes`, `TestExternalBodiesByBinary`, invocação sem flag em `TestRenderFilesPerInvocation` |
| GO-09 | `TestExternalJSONValuesAndStaticBytes`, `TestExternalRenderedOriginAcrossProcessDirectories` |
| GO-10 | `TestExternalRenderedReadAndCloseFailures`, `TestPayloadReadFailureClosesAndDiscardsPartialBytes`, `TestPayloadCloseFailureAndCanceledRead`, `TestExternalRenderedDescriptorClosedBeforeSend`, `TestExternalCancellationAfterRendering` |
| GO-11 | `TestExternalSecretsAndOperationalMetadataNeverReachReports`, `TestExternalRenderOriginIsSanitized`, `TestRenderFilesSecretsAndWriterPriority`, `TestExternalRenderedMixedModesByBinary` |
| GO-12 | `TestExternalRenderedMixedModesByBinary` (inline/external/multipart binário literal) |
| GO-13 | `TestExternalRenderedRedirectsAndTLS`, `TestExternalRenderReporterFailure`, `TestRenderFilesSecretsAndWriterPriority`, `TestExternalRenderedMixedModesByBinary` |
| GO-14 | `TestExternalFileHelp`, `TestExternalRenderedDocumentedScenarios`, `TestMultipartDocumentedScenario`, `TestCompatibility` |


## US-20 — issue #24 / história 020

Plano da issue #24 executado diretamente, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 020 antes da implementação. Tasks 1–3
nos commits `21370fe`, `726729d` e `dc8f93f`; documentação, ajuda, corpus e
validação integrada da task 4 nesta alteração. Sem frontend/estáticos, dependências,
toolchain, CI ou gates alterados. SonarQube não executado: responsabilidade QA.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `1.26.10`, goimports, golangci-lint e Hurl **8.0.0** já disponíveis.
UID 1000: o teste de acesso negado usa processo sem privilégios, arquivo 0000,
código 2, diagnóstico e ausência de contato; fakes cobrem falhas determinísticas.
Servidores HTTP/TLS locais em portas dinâmicas, sem dummy/8080. Cross-builds e
execução nativa macOS/Windows não realizados: opcionais pelo plano v4, seção 7.

- Task 1: `go test ./internal/parser ./internal/runner ./internal/transport` e
  `go test ./tests/e2e -count=1` passaram. POST/PUT/PATCH, zero campos, arquivos
  vazios/binários/placeholders, basename com espaço/Unicode e MIME explícito.
  Preparação não lê a fonte; teste isolado de **64 MiB** usa fonte virtual,
  receptor `io.CopyBuffer` sem acumulação e orçamento de alocação adicional
  inferior a 16 MiB. Sem limite de memória global alterado, sem buffer do arquivo.
- Task 2: `go test ./internal/parser ./internal/template ./internal/runner
  ./internal/cli` e `go test ./tests/e2e -count=1` passaram. Processos filhos
  com cwd distintos comprovam relativo/absoluto e `..` fora do projeto; caminho
  com placeholder é literal. Textos escalares, vazios e composição não recursiva,
  MIME explícito/inferido/fallback, controles/duplicações/bodies misturados,
  Content-Type estático/dinâmico e falhas de arquivo/variável/arrays/objetos.
  Falhas pré-envio preservam contagens anteriores e não expõem textos, arquivo
  ou segredo fictício; respostas de eco/assertions/callbacks continuam sanitizados.
- Task 3: `go test ./internal/runner ./internal/transport ./internal/cli -race`
  e `make e2e` passaram. 307/308 repetem bytes e boundary, com fonte rebobinada
  somente após fechamento do leitor HTTP; detector de leitura/seek concorrente.
  301/302/303 seguem como GET sem body/Content-Type multipart. Dez redirects,
  origem, TLS verificado, deadline único em todos os hops, sem retry. 413 completo
  antecipado passa; status divergente, leitura/fechamento/seek, conexão encerrada,
  timeout e cancelamento retornam suas categorias e impedem próximo envio.
  Testes verificam fechamento antes do callback/avaliação e depois de falhas;
  Ctrl+C real durante upload usa arquivo esparso de 128 MiB e retorna 2.
  Fluxo inline → multipart → externo → inline tipado mantém assertions/capturas;
  falha de writer conserva prioridade de código 2 e não retém descritores.
- Task 4: `go test ./internal/cli ./tests/e2e` passou. Ajuda root/test e exemplo
  `scenario_multipart.hurl` são executados pelo binário com payload adjacente.
  A revisão integrada acrescentou regressão de JSON não vazio após multipart:
  não pode ser confundido com campo textual nem produzir envio parcial. Também
  comprovou que Content-Type multipart preserva parâmetros compatíveis como
  charset, recusando qualquer boundary manual, conforme RN-05.
- `make compatibility` passou sem skip com Hurl **8.0.0**: **116 pares**, dez novos
  `multipart_*`, **106 anteriores inalterados**. Observações semânticas registram
  ordem, campos, filename bruto, MIME, bytes em base64 e contatos/códigos por
  ferramenta, não framing/boundary aleatório. Novos JSONs foram gerados apenas
  com o filtro `^TestCompatibility/multipart_` e revisados; a suíte completa foi
  executada sem `-update`. Diferenças de filename, espaço no caminho, repetição,
  múltiplos arquivos e boundary manual estão em `compatibility.md`. Hurl emite
  aviso local de versão de libcurl, sem impedir identificação 8.0.0 ou testes.
- Fuzz separado com seeds ampliados:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  passou (última execução: **82.350** entradas), e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  passou (**29.671** entradas). Nenhum crash encontrado.
- `make fmt lint test` passou antes de concluir cada task de implementação e na
  validação integrada: lint **0 issues**, testes `-race -cover`. Também passaram
  explicitamente `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
  Builds retornaram sucesso com aviso não fatal de escrita no cache de módulos
  somente leitura do ambiente. `gofmt -l` dos Go alterados e `git diff --check`
  sem pendências. Sem instalação de ferramentas nem flexibilização de gates.

### Rastreabilidade dos cenários da issue

| Cenários | Evidência automatizada |
|---|---|
| GO-01 | `TestMultipartModel`, `TestMultipartWire`, `TestMultipartByBinary`, `TestMultipartDocumentedScenario` |
| GO-02 | `TestMultipartWire`, `TestMultipartByBinary`, `TestMultipartPreparationIsIncrementalAndOwned` |
| GO-03 | `TestMultipartOriginAndMIMEByProcess` (relativo/absoluto/fora da raiz/cwd filho) |
| GO-04 | `TestMultipartTextAndMIME`, `TestMultipartOriginAndMIMEByProcess`, pares de MIME diferencial |
| GO-05 | `TestMultipartTextAndMIME` (escalar/vazio/não recursivo/texto semelhante a JSON) |
| GO-06 | `TestMultipartRejectsStructure`, `TestMultipartInvalidInputsByBinary`, corpus negativo |
| GO-07–08 | `TestMultipartMetadataAndContentType`, `TestMultipartPreparationFailuresPreserveCounts`, corpus negativo |
| GO-09 | `TestMultipartPreparationFailuresPreserveCounts`, `TestMultipartProtectionAndPreparationByBinary` |
| GO-10 | `TestMultipartEarlyResponseAndReadFailure`, `TestMultipartStatusAndDisconnectByBinary` |
| GO-11 | `TestMultipartResourceFailurePaths`, `TestMultipartNetworkPoliciesAndDeadline`, `TestMultipartEarlyResponseAndReadFailure`, `TestMultipartTimeoutByBinary`, `TestRealCtrlC/multipart` |
| GO-12 | `TestMultipartLargeUploadMemory`, `TestMultipartPreparationIsIncrementalAndOwned` |
| GO-13 | `TestMultipartRedirects`, `TestMultipartNetworkPoliciesAndDeadline` |
| GO-14 | `TestMultipartCLISafeDiagnostics`, `TestMultipartProtectionAndPreparationByBinary`, callbacks dos testes runner |
| GO-15 | `TestMultipartMixedBodiesAndCaptures`, `TestMultipartMixedProcess`, `TestMultipartResourceFailurePaths/report` |
| GO-16 | `TestCompatibility`, dez pares multipart com Hurl 8.0.0, diferenças explícitas em `compatibility.md` |

Registro histórico da US-20: renderização externa e `--render-files` não eram
anunciados naquela entrega; a US-10 acima implementa a extensão somente para body
externo completo. Multipart avançado, renderização de arquivo multipart, sandbox
e detecção arbitrária de segredos continuam fora do suporte.

## US-09 — issue #22 / história 009

Plano da issue #22 executado diretamente, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 009. Tasks 1–3 nos commits `cafa469`,
`9d60765` e `19baacf`; contrato, ajuda, corpus e validação integrada da task 4
nesta alteração. Nenhum frontend/estático, dependência, Go, CI ou gate alterado.
SonarQube não foi executado (responsabilidade exclusiva QA).

### Ambiente e comandos efetivamente executados

Linux amd64, Go conforme `go.mod` (`1.26.10`), goimports, golangci-lint e Hurl
8.0.0 já disponíveis, sem instalação. Processo com UID 1000 (não privilegiado):
E2E de acesso negado usa arquivo com modo 0000 e comprova o diagnóstico e ausência
de contato, não apenas a aplicação de chmod. HTTP local em portas dinâmicas,
sem depender de dummy/8080. macOS/Windows nativos e cross-builds são opcionais
pelo plano v4, seção 7; não executados nem anunciados.

- Task 1: `go test ./internal/parser ./internal/runner ./internal/cli
  ./internal/transport` e `go test ./tests/e2e -count=1` passaram. Bytes exatos
  (whitespace, LF/CRLF, não JSON, UTF-8 inválido/binário, zero bytes), nomes com
  espaços/placeholders e caminho absoluto. Malformados/duplicados/misturados e
  headers após body rejeitam todo o cenário; GET sem body preservado.
- Task 2: `go test ./internal/runner ./internal/cli` e
  `go test ./tests/e2e -count=1` passaram. Primeira requisição concluída, falha na
  segunda e terceira não enviada; contagens baseadas no executor. Inexistente,
  diretório, acesso negado real e falhas determinísticas de leitura/fechamento.
  Bytes parciais descartados e causas inspecionáveis via errors.Is/As, sem texto
  bruto. Callback/resultado sem referência operacional/body/headers, escrita
  prevalecendo com código 2, credenciais fictícias ausentes dos streams.
- Task 3: `go test ./internal/runner ./internal/transport ./internal/cli` e
  `make e2e` passaram. Diretório do processo filho distinto sem chdir global,
  referências relativas com `..` fora do projeto e absolutas. Fluxo misto
  inline estático → externo → inline com template, códigos 0/1/2, interrupção.
  Redirects 301/302/303 convertem em GET vazio; 307/308 preservam método/bytes.
  Descritor real fechado antes do executor e após leitura com erro. Resposta
  consumida/fechada em sucesso e falha; cancelamento antes/durante leitura não
  inicia executor e cancelamento no transporte não envia próxima entrada.
  Ctrl+C real agora inclui body externo, com saída 2 e sem envio posterior.
- Task 4: `go test ./internal/cli ./internal/parser ./tests/e2e` passou. Exemplo
  versionado executado pelo binário com payload adjacente, ajuda de root/test,
  negativos no corpus, proteção de headers sensíveis renderizados antes de
  eventual falha local (inclusive quando o mesmo valor ocorre na URL exibível).
- Fuzz: `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s
  -parallel=2` passou: 57.194 execuções, seeds ampliados com referências externas,
  CRLF, caminhos com espaços/placeholders/colon, delimitadores e duplicados;
  nenhuma falha encontrada.
- `make compatibility` passou com Hurl **8.0.0** obrigatório, sem skip: **106
  pares**, seis novos `external_*`; as **100 observações anteriores não mudam**.
  Cada ferramenta recebe sua própria árvore temporária de cenário/payloads,
  sem substituir referências por absolutos. Bytes/contatos/códigos são medidos.
  Espaço literal no nome é aceito aqui e rejeitado neste par pelo Hurl; falhas
  de arquivo retornam 2 aqui e 3 no Hurl observado. JSONs novos escritos/revisados
  explicitamente, sem atualização automática indiscriminada. O binário Hurl
  imprime aviso de metadados de versão da libcurl no ambiente; a identificação
  8.0.0 e todos os pares passaram. Não se anuncia equivalência geral.
- `make fmt lint test` passou antes de concluir cada task e na validação integrada,
  lint **0 issues**, `go test ./... -race -cover`. `go vet ./...` e
  `go build ./...` executados explicitamente. `make build` e `make e2e` passaram.
  `gofmt -l` dos Go alterados e `git diff --check` sem pendências; exemplos,
  links locais, classificações e aviso de ausência de sandbox revisados.

### Rastreabilidade dos cenários da issue

| Cenários | Evidência automatizada |
|---|---|
| GO-01–02 | `TestExternalBytes`, `TestExternalBodiesByBinary`: whitespace, LF/CRLF, binário/não JSON, vazio |
| GO-03–04 | `TestFileBodyLiteral`, `TestExternalBytes`, `TestExternalOriginAcrossProcessDirectories`: origem, absolutos, `..`, nomes literais |
| GO-05 | `TestExternalBodiesByBinary`: mesmo placeholder externo com variável ausente/definida, bytes idênticos |
| GO-06 | `TestInvalidFileBodyRejectsWholeScenario`, `TestMalformedExternalPreventsAllNetwork`, corpus negativo integral |
| GO-07–08 | `TestFileFailuresPreservePreviousResults`, `TestPayloadReadFailureClosesAndDiscardsPartialBytes`, `TestFileDiagnostics`, `TestExternalReadFailureByBinary` |
| GO-09 | `TestExternalBytes`, `TestFileFailureProtectsRenderedSensitiveHeadersBeforeReporting`, `TestFileDiagnostics`, `TestExternalMixedSequenceByBinary` |
| GO-10 | `TestExternalMixedSequenceByBinary` e regressões inline existentes: códigos 0/1/2 e ausência de próximo envio |
| GO-11 | `TestExternalRedirectsByBinary`: cinco status e bytes efetivamente recebidos |
| GO-12 | `TestExternalDescriptorClosedBeforeExecutorOrFailure`, `TestPayloadCloseFailureAndCanceledRead`, `TestExternalResponseConsumedAndClosed` |
| GO-13 | `TestExternalCancellationNeverStartsPrematurely`, `TestRealCtrlC/external`, `TestExternalMixedSequenceByBinary/incomplete` |
| GO-14 | `TestCompatibility`, seis pares `external_*`, Hurl 8.0.0 sem skip; diferenças em `compatibility.md` |

Registro histórico da US-09: renderização de arquivos externos, `--render-files`,
multipart e sandbox estavam fora do suporte daquela entrega. Multipart foi
ampliado na US-20 acima; evidência local não substitui review.

## US-08 — issue #20 / história 008

Plano da issue #20 executado diretamente, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 008 antes de implementar. Tasks 1–3
nos commits `8f2b454`, `4dc224d` e `da677d9`; contrato, ajuda, corpus e validação
da task 4 nesta alteração. Nenhum frontend/estático, dependência, versão de Go,
CI ou lint alterado. Não foi executado SonarQube (responsabilidade exclusiva QA).

### Ambiente e comandos efetivamente executados

Linux amd64; Go `go1.26.10-X:nodwarf5`, CGO/compilador C para race, goimports,
golangci-lint e Hurl 8.0.0/libcurl 8.18.0 já disponíveis, sem instalação. HTTP/TLS
local em portas dinâmicas, sem depender de dummy/8080. Conforme plano v4, seção 7,
cross-builds e execução nativa macOS/Windows são opcionais nesta história; não
foram executados nem anunciados. Linux é o gate obrigatório.

- Task 1: `go test ./internal/query ./internal/assertion ./internal/parser` passou.
  Lista ordenada com variantes concretas; JSONPath validado antes da rede;
  literais tipados, escapes/Unicode e placeholders literais; erros tardios,
  operadores/queries fora do recorte e posições LF/CRLF. Seleção detalhada separa
  presença/ausência/percurso incompatível e mantém o adapter das capturas.
  Igualdade recursiva sem coerção/float64; normalização decimal compara números
  grandes e expoentes além do inteiro de máquina, sem expandir potências.
- Task 2: `go test ./internal/template ./internal/runner ./internal/cli` e
  `go test ./internal/parser ./internal/query ./internal/assertion` passaram.
  Status → assertions na ordem escrita → capturas, decode sob demanda, retenção
  só quando necessária, códigos 0/1/2, interrupção e contagens. Callback recebe
  somente metadados/diagnósticos seguros: nenhum literal JSON esperado bruto,
  body, consulta compilada ou documento. Captura secreta pendente oculta ambos
  os operandos de headers/JSON, até quando sua própria consulta falharia, sem
  antecipá-la. Cenário não mutado, publicação atômica e sessões isoladas mantidos.
- Task 3: `go test ./internal/transport` e `go test ./tests/e2e -count=1` passaram.
  Build único do binário, CommandContext, HTTP real, contadores e sincronização
  sem sleeps. Demonstração e login → token protegido → criação → ID exato
  `9007199254740993` → consulta com headers/JSON → echo numérico. Tipos/precisão,
  estruturas, literais e falhas verificam stdout/stderr/exit code e envios.
  Apenas body final de redirect é avaliado; falhas de leitura/fechamento, body
  incompleto, timeout/cancelamento impedem aprovação/envio seguinte, sem retry.
  Timeout público real de 10s e Ctrl+C Linux agora incluem assertion JSON;
  regressões de transporte com capturas continuam. Pipe comprova resultado com
  assertion JSON/captura visível enquanto o envio seguinte está bloqueado.
- Fuzz: `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s
  -parallel=2` passou, 61.545 execuções, novas seeds de literais/assertions JSON,
  CRLF e negativos. Nenhum caso de falha encontrado.
- `make fmt lint test` passou antes de concluir cada task anterior e na validação
  integrada: **0 issues** de lint, `go test ./... -race -cover`. A suíte completa
  foi repetida sem cache com `go test ./... -race -cover -count=1`, passando
  inclusive pelos E2E do binário atualizado. `go vet ./...` e `go build ./...`
  executados explicitamente; `make build` também passou.
  `gofmt -l` dos Go alterados e `git diff --check` sem pendências. Links Markdown
  locais conferidos, 100 pares JSON/classificações conferidos e somente o par
  antigo autorizado tem observações alteradas.
- `make compatibility` passou com Hurl **8.0.0** obrigatório, sem skip:
  **100 pares**, 21 novos `assert_json_*`. `headers_query_unsupported.json` muda
  intencionalmente de rejeição para sucesso; os outros **78 pares antigos não
  mudam**. Observações novas escritas/revisadas individualmente, sem `-update`
  indiscriminado. Primeira rodada revelou rejeição do literal esperado `125e-2`
  no Hurl e aceitação do wildcard; somente essas duas novas observações foram
  corrigidas e a matriz completa reexecutada. Arrays/objetos esperados também
  são rejeitados no Hurl observado, sem alterar a regra aprovada aqui. A matriz
  distingue códigos/categorias, literal não interpolado, precedência de captura
  e restrições, sem prometer equivalência geral ou mensagens iguais.

### Rastreabilidade dos cenários da issue

| Cenários | Evidência automatizada (nível pertinente) |
|---|---|
| GO-100 | `TestJSONDemonstrationByBinary`, `TestJSONAssertionOrder`, `TestJSONAssertionCLI`; fixture `scenario_assert_json.hurl` |
| GO-101 | `TestJSONAuthenticatedProductsByBinary`, `TestJSONLiteralAndCapturePublication`; fixture autenticada com ID de alta precisão e headers |
| GO-102 | `TestJSONEqual`, `TestTypedSelectionPrecisionAndPresence`, `TestJSONEqualityAndEvaluationByBinary`: seis tipos/vazios, raiz/propriedades/índices e não coerção |
| GO-103–104 | `TestJSONEqual`, `TestJSONEqualityAndEvaluationByBinary`: números exatos/expoentes grandes, tipos/conteúdo aninhado, ordem de arrays e chaves de objetos |
| GO-105–107 | `TestDetailedSelection`, `TestJSONAssertionOrder`, `TestJSONAssertionCLI`, `TestJSONEqualityAndEvaluationByBinary`: ausência/null, tipos incompatíveis, JSON vazio/inválido/adicional e categorias |
| GO-108–109 | `TestInvalidJSONAssertions`, `TestJSONAssertions`, `TestJSONInvalidLateNeverSendsByBinary`, `TestJSONLiteralAndCapturePublication`: zero envios, consultas/operadores, LF/CRLF, literais/Unicode sem renderer |
| GO-110–111 | `TestJSONAssertionOrder`, `TestJSONPrecedenceByBinary`, `TestJSONLiteralAndCapturePublication` e regressões de publicação atômica: primeira falha, status/HTTP *, captura apenas após assertions |
| GO-112 | `TestJSONAssertionRequiresSuccessfulReadAndClose`, `TestJSONAssertionFinalBodyDeadlineAndCancellation`, `TestJSONFinalResponseAndIncompleteBodyByBinary`, `TestPublicTimeoutIncludesBody`, `TestRealCtrlC`; regressão de headers sem retenção e deadlines únicos do transporte |
| GO-113–115 | `TestJSONKnownSecretsAreSanitizedBeforeCallback`, `TestPendingSecretConcealsAssertionsAndMetadata`, `TestJSONAndHeaderPendingSecretCLI`, `TestJSONSecretsByBinary`; regressões de variantes/derivados em template e capturas tipadas |
| GO-116 | `TestJSONWriteFailureHasPriority`, `TestJSONAssertionOrder`, `TestResultsStreamWhileNextRequestIsPending`: callback síncrono, contagens/interrupção, erro/short write com prioridade 2 |
| GO-117 | `TestCompatibility`, 100 pares, Hurl 8.0.0 sem skip; diferenças classificadas em `compatibility.md` |
| GO-118 | `TestJSONAssertionHelp`, `TestHeaderHelpContract`, regressões de ajuda/versão/flags/streams/PTY/NO_COLOR; exemplos JSON executados pelo binário |

Esta evidência local não é aprovação de review. O handoff final solicita revisão
pelo code-reviewer; não se declara compatibilidade Hurl completa.

## US-07 — issue #18 / história 007

Plano executado diretamente da issue #18, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 007 antes da implementação. Tasks 1–3
concluídas nos commits `e5ed98c`, `87026b9` e `1e18abf`; a implementação e evidência
local da task 4 estão nesta alteração. Nenhum frontend/estático, dependência,
toolchain, CI ou lint alterado. SonarQube não foi executado (exclusivo do QA).

### Ambiente e comandos efetivamente executados

Linux amd64; Go `go1.26.10-X:nodwarf5`, CGO/compilador C para race, goimports,
golangci-lint e Hurl 8.0.0/libcurl 8.18.0 já disponíveis; nenhuma instalação.
Servidores HTTP/TLS locais em portas dinâmicas, sem dummy ou serviço em 8080.
Linux é o gate obrigatório do plano v4, seção 7. Cross-builds e runtime nativo
macOS/Windows são opcionais nesta história, não foram executados nem anunciados.

- Task 1: `go test ./internal/parser ./internal/runner ./internal/cli` e
  `go test ./tests/e2e -count=1` passaram. `HTTP *` em qualquer entrada aceita
  200/404/503, sem status fictício ou comparação numérica contada. Ausência de
  resposta permanece erro de transporte; expectativa ausente/numérica preservadas.
- Task 2: `go test ./internal/parser ./internal/query ./internal/assertion
  ./internal/runner ./internal/transport ./internal/cli` passou. Três operadores,
  caixa de nome/valor, primeira ocorrência, vírgulas, ausência/vazio/substring
  vazia, esperado literal com variável definida/ausente, seções e posições LF/CRLF.
  Fakes verificam status → headers → capturas, ausência de publicação/envio após
  falha, callback síncrono e resultados sem coleção bruta de headers/body.
  Valores sensíveis literais e recebidos ficam ocultos antes do callback; headers
  comuns também usam segredos conhecidos. Escrita/short write prevalece com 2.
- Task 3: `go test ./internal/transport` e `go test ./tests/e2e -count=1` passaram.
  Binário real, compilado uma vez: demonstração, cada operador satisfeito/falho,
  ausência/vazio/repetição/vírgula, códigos/streams e interrupção preservando
  resultados prévios. Capturas tipadas com `HTTP *` em 200/404/503; status ou
  header divergente prevalece sobre body inválido. Erro tardio impede todos os
  envios. Saídas não expõem esperado/recebido sensíveis. Só headers finais de
  redirect satisfazem assertions. Body incompleto e leitura/fechamento com erro
  não aprovam headers nem liberam próxima requisição; sem capturas, não há
  retenção. Timeout público de 10s e Ctrl+C real Linux exercitam headers corretos
  antes de leitura/cancelamento. Pipe comprova resultado com assertion/captura
  visível antes de terminar o próximo envio, sem sleeps.
- `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=10s` passou:
  257.262 execuções, seeds ampliados para curinga/assertions/transições e negativos;
  nenhuma falha encontrada.
- `make fmt lint test` passou antes de cada task concluída e na validação final:
  **0 issues** no lint, `go test ./... -race -cover`. `go vet ./...` executado
  explicitamente em cada entrega de código. `go build ./...` e `make build`
  também passaram na validação integrada. `gofmt -l` e `git diff --check` sem
  pendências; links Markdown locais e JSONs/classificações dos pares conferidos.
- `make compatibility` passou com Hurl **8.0.0** obrigatório, sem skip, **79 pares**.
  **21 pares novos** `headers_*`; os **58 anteriores não mudaram**. Nenhuma
  atualização automática indiscriminada: observações novas escritas e revisadas
  individualmente. A primeira execução identificou que Hurl retorna lista para
  repetidos, não a primeira string; isso foi classificado, sem mudar a regra
  HTTPGo. Também observados esperado interpolado no Hurl (inclusive erro de
  variável ausente com exit 4) e captura antes de assertion (exit 3 contra 1).
  As classificações foram corrigidas conforme evidência e a matriz foi
  reexecutada sem `-update`. `docs/compatibility.md` distingue equivalência
  compartilhada, restrições e divergências deliberadas; não presume mensagens
  ou motores iguais. Ajuda da CLI e demonstrações têm testes próprios.

A evidência local não é aprovação de review. O handoff final solicita revisão
pelo code-reviewer; nenhuma alegação de compatibilidade Hurl completa.

## US-06 — issue #16 / história 006

Plano executado diretamente da issue #16, sem planner, somente backend. Três
tasks registradas e autoaprovadas na fila 006 antes da implementação. Tasks 1 e 2
concluídas com commits `3afeb3b` e `fe893b6`; implementação e validação da task 3
registradas nesta alteração. Sem frontend/estáticos, novas dependências, mudança
de toolchain, CI ou lint. SonarQube não foi executado (exclusivo do QA).

### Política atual de portabilidade

Conforme o plano v4, seção 7, referenciado na issue #16, **Linux é o gate obrigatório
por história**. `make portable` e testes nativos macOS/Windows são opcionais nesta
história e ficam para a validação final de portabilidade/release. Não foram
executados nesta US-06 e sua ausência **não bloqueia** tasks, review ou conclusão.
Nenhuma execução nessas plataformas é declarada. Os registros US-01–US-05 abaixo
são históricos: suas afirmações de bloqueio nativo não definem o gate atual e
não devem ser interpretadas como instruções para impedir o handoff da US-06.
Build cruzado, quando executado, não é evidência de runtime.

### Ambiente e comandos efetivamente executados

Linux amd64; Go `go1.26.10-X:nodwarf5`, CGO/compilador C para race, goimports,
golangci-lint e Hurl 8.0.0/libcurl 8.18.0 já disponíveis. Nenhuma instalação.
Servidores HTTP/TLS locais com portas dinâmicas, sem depender de dummy/8080.

- `go test ./internal/query ./internal/template`: todos os tipos na raiz,
  propriedades/índices, presença de null/vazios versus ausência; números grandes,
  decimais e expoentes, inclusive aninhados, sem float64. Publicação atômica,
  cópia do payload JSON, erro seguro sem fallback textual, strings semelhantes
  a JSON/placeholders, objetos opacos, conflitos exatos/hierárquicos e proteção
  de constituintes mesmo quando o lote falha. Sessão e contexto inicial isolados.
- `go test ./internal/runner ./internal/cli ./internal/parser ./internal/transport`:
  demonstração autenticada com ID `9007199254740993` exato no path e JSON numérico;
  callback síncrono, contagens, cenário não mutado e novo Run sem capturas.
  Contextos textuais escalares, estruturas rejeitadas antes do envio com posição
  original LF/CRLF; arquivo inválido/não suportado rejeitado antes da rede.
  JSON inválido/adicional, ausência e percurso incompatível, status divergente
  antes da captura, 4xx esperado e ausência de expectativa. Saídas protegidas em
  sucesso, divergência, captura/conflito, renderização, transporte, erro de escrita
  e short write; escrita prevalece também sobre divergência. HTTP/TLS real retém
  só resposta final; body tipado completo seguido de falha de leitura/fechamento,
  cancelamento/timeout ou resposta incompleta não publica nem libera novo envio.
- `go test ./tests/e2e -count=1`: passou pelo binário real com build único e
  CommandContext. Cenários `scenario_capture_products.hurl` e
  `scenario_capture_auth_products.hurl` comprovam os três status da demonstração
  e o login adicional, token efetivamente recebido, precisão e isolamento entre
  processos. Todos os tipos, vazios, números aninhados e strings literais são
  selecionados na raiz/propriedades/índices, verificando bytes sem float64.
  Escalares em URI/header/string JSON e encoding explícito são observados pela
  API. Array/objeto em URL/header/string JSON retorna 2 com localização LF/CRLF,
  uma resposta concluída e zero envios dependentes/posteriores. Segredos escalares
  e compostos, constituintes e escapes conhecidos permanecem ocultos nos streams
  e filenames, inclusive após outra declaração do mesmo lote falhar; dados
  enviados continuam intactos. Redirects 301/302/303/307/308 usam apenas token,
  número e array da resposta final, com método/body verificados. Regressões de
  ajuda/versão, flags, pipe/arquivo/PTY Linux, NO_COLOR, timeout público de 10s,
  cancelamento real, rede e apresentação incremental passaram.
- `make compatibility`: passou, **58 pares**, Hurl **8.0.0** obrigatório, sem skip.
  Oito pares `captures_typed_*` novos; `captures_non_string.json` muda
  intencionalmente de rejeição para sucesso com header `123`. Os outros **49 pares
  anteriores não mudaram**. Observações atualizadas apenas nos pares autorizados
  com `-update`, lidas/revisadas e reexecutadas sem atualização. URI/header/body
  comprovam ID `9007199254740993`, booleano e null compartilhados nesses pares;
  estruturas completas passam aqui e terminam com 3 no Hurl. Diferenças e limites
  documentados em `compatibility.md`, sem presumir equivalência geral de tipos.
- `make fmt lint test`: passou, **0 issues** no lint e `go test ./... -race -cover`.
  Também executado antes de concluir cada uma das tasks 1 e 2.
- `go vet ./...`, `go build ./...` e `make build`: passaram.
- Fuzzing separado, 20 segundos e duas workers por alvo, com seeds ampliados:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    passou; 13.541 execuções.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`:
    passou; 30.376 execuções, incluindo capturas string e JSON tipado válido.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    passou; 693.829 execuções. Nenhum caso de falha encontrado.

A evidência local não é aprovação de review: o handoff final da fila 006 solicita
a revisão da história; o veredito cabe ao code-reviewer.

## US-05 — issue #14 / história 005

Plano executado diretamente da issue #14, sem planner, somente backend. Tasks
registradas e autoaprovadas na fila 005 antes da implementação. Tasks 1–3
concluídas com commits `15961e8`, `58de83b` e `9f82684`. A implementação e a
validação **local** da task 4 estão nesta alteração. Seu aceite continua
**bloqueado pela execução nativa macOS/Windows** exigida no plano. Nenhuma
alteração de frontend/estáticos, dependências, versão de Go, CI ou lint.
SonarQube não foi executado (exclusivo do QA).

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`, CGO/compilador C para race,
goimports/golangci-lint e Hurl 8.0.0/libcurl 8.18.0 já disponíveis. Nenhuma
ferramenta foi instalada. Os servidores dos testes são HTTP/TLS locais, em
portas dinâmicas; não há dependência do dummy nem de serviço externo.

- `go test ./internal/parser ./internal/query`: associação e posições de
  declarações/consultas, LF/CRLF, limites com JSON/strings e próximas entradas,
  rejeição integral de seção órfã/vazia/duplicada, nomes e queries inválidos ou
  não suportados. Seleção de raiz, propriedades/índices combinados, vazio,
  ausência versus null, tipos preservados com `UseNumber` e erros sem resposta.
- `go test ./internal/template ./internal/runner ./internal/transport`: sessão
  nova por Run, strings sem reinterpretar literais CLI/placeholders, nomes exatos,
  publicação atômica e colisões hierárquicas em qualquer tipo inicial; fakes HTTP
  sem rede no runner, contagens e callback síncrono. HTTP local comprova login,
  Authorization e somente bytes finais de redirects retidos. Sem captura não há
  retenção. Leitura incompleta, cancelamento/timeout durante leitura e falha de
  fechamento não publicam capturas nem liberam próximos envios. Falhas de leitura
  e fechamento também são injetadas por RoundTripper de teste com body observável.
- `go test ./internal/cli ./internal/template ./internal/runner`: códigos 0/1/2,
  captura localizada separada de transporte/renderização, status divergente
  prevalece sobre JSON inválido, 4xx esperado e ausência de expectativa. Proteção
  inicial/dinâmica, segredo descoberto antes de falha em outra declaração,
  variantes brutas/JSON/Go/URL conhecidas, filename escapado, strings semelhantes
  a JSON, vazio, isolamento do comando e ausência de corpos/causas brutas.
  Writers com erro/short write impedem próximo envio; escrita prevalece sobre
  divergência. Contagens mantêm resposta concluída e status aprovado na falha
  de captura, sem inventar transporte ou reimprimir resultados.
- `go test ./tests/e2e -count=1`: passou com binário real compilado uma vez.
  Demonstração `scenario_auth_capture.hurl` verifica credenciais fictícias e
  Bearer efetivamente recebido; segundo processo/arquivo não encontra a captura.
  POST/PUT/PATCH/DELETE com strings completas/compostas, arrays/caminhos aninhados,
  nomes pontuados, strings vazias, controles/Unicode, escapes e texto semelhante
  a placeholder. Casos negativos verificam stdout/stderr, exit code e contatos:
  tipo/JSON/caminho, status, colisões iniciais/de lote/anteriores, uso antecipado,
  URL/header final inválido e construção tardia inválida/não suportada com zero
  envios. Proteção de senha/token também em nomes de arquivo e falhas.
- Regressões E2E de rede com capturas: redirects 301/302/303/307/308 retêm somente
  JSON final; origem bloqueada, TLS inválido, body incompleto e conexão encerrada
  não liberam próxima entrada nem retry. GET com captura comprova timeout público
  de **10 segundos** incluindo redirect e leitura; POST tipado sem captura conserva
  o teste anterior. Ctrl+C real Linux interrompe GET durante leitura de body com
  captura; métodos restantes conservam cancelamento sem captura. Pipe de stdout
  comprova resultado do login visível enquanto POST autenticado seguinte está
  bloqueado, em sucesso, divergência e body incompleto. Ajuda/versão, flags
  inválidas, NO_COLOR, arquivo/pipe/PTY Linux continuam passando.
- `make compatibility`: passou com **50 pares**, Hurl **8.0.0** obrigatório e sem
  skip. Nove pares novos `captures_*` foram atualizados com `-update`, lidos e
  revisados, e novamente executados sem atualização. Confirmam token recebido
  pela API e diferenças de nomes pontuados, sobrescrita, captura não string,
  ausência de expectativa e string `"null"` em valor JSON completo. Os **41 pares
  antigos não mudaram**; o harness acrescenta somente campos opcionais de resposta
  JSON/Authorization. A troca do negativo genérico de Captures por seção vazia e
  negativos específicos é uma ampliação intencional documentada do subconjunto.
- `make fmt lint test`: passou; lint com **0 issues**, `go test ./... -race -cover`.
  O aviso inicial de lint no novo teste foi corrigido antes da rodada aprovada.
- `go vet ./...`, `go build ./...`, `make build` e `make portable`: passaram.
  Portable comprova somente build CGO=0 linux/darwin/windows amd64, não runtime.
- Fuzzing separado, 20 segundos e duas workers por alvo:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    passou; 700.202 execuções, seeds ampliados para seções/consultas/fronteiras.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`:
    passou; 41.010 execuções, também verificando strings provenientes de captura.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    passou; 430.095 execuções, preservando tipo/conteúdo da captura, JSON válido e
    origem sem mutação. Nenhum panic ou caso de falha foi encontrado.

### Bloqueio de aceite / handoff final da 005

Não há ambiente nativo macOS/Windows nesta sessão. Falta executar
`go test ./tests/e2e -count=1` nativamente em cada plataforma e registrar os
resultados aplicáveis de streams/códigos/contatos. Permanecem os limites históricos:
harness de sinal real com tags linux/darwin, executado apenas em Linux, e PTY
Linux-only; não há evidência de console/sinal/TTY nativos nas outras plataformas.
**Cross-build e testes de contexto não substituem essa evidência.**

A task `back:4` não deve receber `done`; seu claim deve ser liberado enquanto
aguarda ambientes/resultados. Não houve handoff final, disparo de review nem
comentário de implementação concluída na issue #14. A implementação passa nos
gates locais, mas a história não está declarada pronta. Nenhum gate foi removido,
reduzido ou transformado em skip. As tasks 1–3 permanecem concluídas.

## US-04 — issue #13 / história 004

Plano da issue #13 executado sem planner, somente backend. As quatro tasks foram
registradas e autoaprovadas na fila 004 antes de implementar. Tasks 1–3 concluídas
com commits `31b7b72`, `145ecbc` e `e3d474d`. A implementação e os testes locais da
task 4 estão nesta alteração; **seu aceite está bloqueado pela evidência nativa
macOS/Windows**, conforme o critério do plano. Não há frontend/estáticos, novas
dependências, aumento de Go ou mudanças de CI/lint. SonarQube não foi executado.

### Ambiente e evidência local executada

Linux amd64, Go `go1.26.10-X:nodwarf5`, CGO/GCC para race, goimports e
golangci-lint já disponíveis; Hurl 8.0.0/libcurl 8.18.0. Nenhuma ferramenta foi
instalada. Os testes usam HTTP/TLS local em portas dinâmicas, não o dummy.

- `go test ./internal/template`: literais e fallback textual, strings decodificadas,
  vazio, `=`, números grandes/exponentes sem float64, objetos opacos, nomes exatos,
  conflitos, representações escalares e rejeição de estruturas em texto.
- `go test ./internal/parser ./internal/template`: plano de referências JSON,
  valores completos/raiz, composição de strings, escapes/Unicode, não recursão,
  estrutura completa validada antes da rede, chaves estáticas, posições LF/CRLF,
  fronteiras e bytes estáticos (inclusive fechamentos adjacentes e `}}` em string).
- `go test ./internal/runner ./internal/cli ./internal/transport`: fake sem rede,
  contrato HTTP real, demonstração com tipos, clones/cenário imutável, apresentação
  síncrona e resultados anteriores mantidos após falha. Variável ausente, tipo
  incompatível e controles decodificados impedem o envio afetado/posteriores.
  Renderer falso com JSON final inválido comprova a defesa pré-executor, com erro
  seguro e posição no início do body, sem propagar payload. O renderer real insere
  JSON válido por construção e ainda o valida antes de retornar.
- Proteção antes/depois do contexto: valores fictícios brutos, decodificados,
  escapes JSON/Go/URL conhecidos, componentes de estruturas, filename já escapado
  seguido de quoting de diagnóstico, segredo vazio, ordem de flags e isolamento.
  Sucesso, divergência, uso, estrutura, renderização, transporte e falha de escrita
  não reproduzem payloads ou causas sensíveis.
- `go test ./tests/e2e -count=1`: passou pelo binário real (build único).
  Demonstração `scenario_products_typed.hurl`, tipos completos inclusive raiz,
  precisão, escapes e Unicode, não recursão, ausência/conflitos/objetos opacos,
  invalidez tardia com zero envios, interrupção preservando resumo e localização
  original após expansão/escape, bytes estáticos LF/CRLF e saídas protegidas.
- Regressões E2E com **body renderizado**: redirects 301/302/303/307/308 com método
  e bytes verificados; falhas HTTP/TLS/origem/limite/conexão/body incompleto,
  ausência de retry, timeout público real de 10 segundos incluindo redirect/body
  e Ctrl+C real Linux. Pipe confirma resultado anterior visível enquanto POST
  tipado seguinte está bloqueado. Testes de escrita com streams injetados mantêm
  prioridade do código 2 e impedem o envio seguinte. Ajuda/versão/flags inválidas,
  NO_COLOR, arquivo/pipe/PTY Linux continuam passando.
- `make compatibility`: passou com **41 pares**, Hurl **8.0.0** obrigatório,
  sem skip. `variables_body.json` muda intencionalmente de rejeição para sucesso;
  seis pares `typed_*` distinguem semântica compartilhada de extensões CLI. As
  observações geradas com `-update` foram lidas e revisadas antes da rodada sem
  atualização. Os outros 34 pares antigos conservam suas observações.
- `make fmt lint test`: passou; lint com **0 issues**, testes `-race -cover`.
- `go vet ./...`, `go build ./...`, `make build` e `make portable`: passaram.
  Builds emitiram aviso de cache de metadados do módulo em diretório somente
  leitura, sem impedir compilação (exit 0). `make portable` comprova apenas build
  CGO=0 linux/darwin/windows amd64, não execução nativa nas outras plataformas.
- Fuzzing separado, 20 segundos e duas workers por alvo, repetido após os ajustes:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    passou; 18.927 execuções na última rodada.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`:
    passou; 17.232 execuções na última rodada.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`:
    passou; 463.924 execuções na última rodada; nenhum panic/JSON inválido aceito
    ou mutação da origem encontrado.

### Bloqueio de aceite / handoff final da 004

Esta sessão não dispõe de runtime nativo macOS/Windows. A lacuna herdada das
histórias anteriores não foi resolvida nem convertida em skip. Falta executar
`go test ./tests/e2e -count=1` nativamente nessas plataformas e registrar a evidência
aplicável de streams/códigos/contatos; os harnesses de sinal/PTY têm os limites
históricos descritos abaixo. **Cross-build não substitui execução nativa.**

A task `back:4` deve permanecer não concluída na fila até suprir a evidência
exigida; o claim é liberado enquanto aguarda ambientes/resultados. Não enviar
`done` final, não disparar review e não publicar comentário de implementação
concluída na issue #13 antes de resolver o bloqueio. A implementação passa nos
gates locais, mas a história não está declarada pronta. Commits da task 4 são
consultáveis no histórico do repositório. Nenhum gate foi removido ou reduzido.

## US-03 — issue #11 / história 003

Plano executado diretamente da issue #11, sem planner, somente backend. Quatro
tasks registradas e autoaprovadas na fila 003. Tasks 1–3 implementadas, validadas
e commitadas (`3900179`, `8330558`, `ed8523e`); implementação e validação local da
task 4 estão registradas nesta alteração. Nenhum frontend/estático, dependência,
versão de Go ou configuração CI/lint foi alterado.

Ambiente efetivamente usado: Linux amd64, Go `go1.26.10-X:nodwarf5`, CGO/GCC
para race, ferramentas goimports/golangci-lint já disponíveis, Hurl
8.0.0/libcurl 8.18.0. Não foi necessário instalar ferramentas. Não foi executado
SonarQube (exclusivo do agente QA).

### Evidência local executada

- `go test ./internal/parser ./internal/template`: nomes, valores textuais,
  conflitos em qualquer ordem, vazio versus ausência, `=`, offsets, substituição
  não recursiva, proveniência e rejeição integral de sintaxe inválida/body.
- `go test ./internal/runner ./internal/parser ./internal/transport`: executor
  falso sem rede e HTTP real local; URL/header final, Host, preservação de bytes
  e cenário original, erro pré-envio localizado sem incrementar Started,
  resultados anteriores e callback síncrono, interrupção de todas as seguintes.
- `go test ./internal/cli ./internal/runner ./internal/template`: flags repetíveis,
  contexto novo por execução, ordem de declarações, ajuda e diagnósticos seguros,
  proteção inclusive antes de construir contexto e prioridade de falhas de escrita.
- `go test ./tests/e2e -count=1`: binário real compilado uma vez por suíte,
  demonstração em dois destinos sem alterar arquivo, headers confirmados pelo
  servidor, valores fictícios protegidos inclusive no path, texto vazio, números/
  booleanos, `=`, não recursão e JSON estático. Erros de flags/estrutura impedem
  todos os envios; erro de resolução/validação conserva primeira resposta,
  não conta a entrada afetada e interrompe as demais. Encoding inválido inclusive
  em query e CR/LF/control em header são recusados antes da rede.
- Regressões E2E passaram também com URL/headers parametrizados: redirects
  301/302/303/307/308, bytes e métodos, TLS inválido, conexão, origem/limite,
  resposta incompleta, ausência de retry, timeout público de 10 segundos
  incluindo body, Ctrl+C real Linux e stdout incremental enquanto a próxima
  requisição está bloqueada. Pipe, arquivo, PTY Linux e NO_COLOR passam.
- `make compatibility`: **35 pares** contra Hurl **8.0.0**, sem skip. Dez pares
  novos sinalizam ampliação intencional do contrato; os 25 antigos não mudaram.
  Variáveis são fornecidas às duas ferramentas pelo harness, sem editar o cenário
  parametrizado; RequestURI comprova encoding explícito. Diferenças observadas
  (nomes pontuados, sobrescrita, body e header dinâmico) em `compatibility.md`.
- `make fmt lint test`: passou com 0 issues de lint e `go test ./... -race -cover`.
- `go vet ./...`, `go build ./...`, `make build`: passaram.
- `make portable`: passou, sem CGO, linux/darwin/windows amd64; **somente build**.
- Fuzzing separado (20 segundos, duas workers por alvo):
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
    passou; 8.809 execuções na rodada, sem falha encontrada.
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`:
    passou; 1.199.362 execuções, sem falha encontrada.
- `gofmt -l` nos Go alterados e `git diff --check`: sem pendências.

### Bloqueio de aceite nativo / handoff final

Não há runtime nativo macOS/Windows nesta sessão. A task 4 exige E2E nativo por
plataforma antes de declarar seu aceite. **Cross-build não resolve essa lacuna.**
Logo a task 4 não deve receber `done`, nem disparar revisão final/conclusão da
issue #11, até disponibilizar os ambientes/evidências requeridos. Essa pendência
não foi convertida em skip ou removida do plano. As tasks anteriores permanecem
concluídas; o código local passa, mas a história não está declarada pronta.

Próximas validações necessárias, não executadas aqui: suíte
`go test ./tests/e2e -count=1` nativa em macOS e Windows, com comprovação dos
contratos de streams/códigos/contatos. O harness de sinal atual é linux/darwin,
e o de PTY é Linux-only; não há alegação de sinal/TTY nativos Windows/macOS.
A matriz histórica de runtime abaixo continua pendente nessas plataformas.

## US-02 — issue #6 / história 002

Escopo somente backend, seguindo as três tasks da issue #6. Nenhuma dependência
de produto, versão de Go, configuração CI/lint ou frontend/estático foi alterado.
A aprovação final é responsabilidade do reviewer; esta seção registra evidência
local, não aprovação ou aceite nativo de outras plataformas.

Ambiente: Linux amd64; Go `go1.26.10-X:nodwarf5`, CGO habilitado para race, GCC e
headers C disponíveis. Hurl 8.0.0/libcurl 8.18.0. `goimports v0.52.0` e
`golangci-lint v2.15.0` foram instalados com autorização nesta sessão, sem alterar
`go.mod`. Comandos de desenvolvimento usam `$(go env GOPATH)/bin` no PATH.

Validações da implementação:

- `go test ./internal/parser`: bytes exatos de objetos/arrays/escalares, LF/CRLF,
  limites de body, headers, erros seguros e rejeição integral.
- `go test ./internal/runner ./internal/transport`: fake de orquestração e HTTP/TLS
  reais; métodos, headers repetidos/Host, bytes, sequência, sem expectativa,
  interrupção, redirects 301/302/303/307/308, limite exato 10/11 e origem bloqueada;
  timeout único entre redirects/body, cancelamento, TLS inválido e sem retry.
- `go test ./internal/cli ./tests/e2e`: método correto nos resultados, códigos
  0/1/2, exemplo JSON versionado contra servidor local e binário real, sequência
  mista, JSON escalar/multilinha, falha tardia com zero envios e proteção de valores
  em Authorization/Proxy-Authorization/Cookie/Set-Cookie/body em sucesso e erros.
- E2E de transporte cobre GET/POST/PUT/PATCH/DELETE em falha, TLS, origem, limite,
  protocolo/body incompleto e sem retry; timeout público real de **10 segundos**
  comprovado para GET e POST, incluindo redirect 307 e leitura do body.
- Ctrl+C real no Linux para os cinco métodos; próximas entradas não iniciadas.
  Regressões de ajuda, versão, stdout incremental, falha de escrita, NO_COLOR,
  pipe/arquivo/PTY Linux continuam presentes.
- `make fmt lint test`: passa, incluindo `go test ./... -race -cover`.
- `go vet ./...`, `go build ./...`, `make build`: passam.
- `make portable`: compila sem CGO para linux/darwin/windows amd64; somente build,
  não execução nativa de macOS/Windows.
- `make compatibility`: passa contra Hurl 8.0.0, **25 pares**, sem skip;
  novos pares comparam método, bytes e headers no servidor. Mudança intencional
  de contrato: `post.json` agora espera POST aceito. Os demais pares US-01 não
  mudam. Diferença de Content-Type automático documentada em `compatibility.md`.
- `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`:
  passa, aproximadamente 798 mil execuções nesta sessão, sem falha encontrada.
- `gofmt -l` nos Go alterados e `git diff --check`: sem pendências.

Os testes locais não dependem do dummy nem de serviço fixo na porta 8080.
Por solicitação posterior, o dummy manual passou a aceitar a demonstração POST
em `/products`, operações em `/products/1` e `/echo`, sem persistência. O cenário
`scenario_dummy_methods.hurl` permite verificar os métodos com `make serve`.
Os testes opt-in do utilitário exercitam HTTP local e os quatro cenários
publicados pelo binário real, sem ocupar 8080; comandos em `dev-server.md`.
Não foi executado SonarQube (exclusivo do QA). Builds cruzados, se executados,
continuam sendo apenas evidência de compilação; não se declara execução US-02
em macOS/Windows. As limitações nativas herdadas da US-01 estão descritas abaixo.

## Histórico US-01 — issue #4

O texto abaixo preserva a evidência e o estado registrados na entrega anterior.
O gate local com race passou após disponibilizar os pré-requisitos C na sandbox;
naquele registro não havia handoff final e o runtime macOS/Windows estava pendente.
Não há skip silencioso, retirada de checks ou redução de cobertura para declarar
sucesso. Não foi executada análise SonarQube (responsabilidade QA).

## Ambiente e ferramentas

- Host de execução: Linux amd64, sandbox.
- Módulo existente: `capimgrosso.com/httpgo`; Go `1.26.10` conforme `go.mod`.
- Go observado: `go1.26.10-X:nodwarf5 linux/amd64`.
- Ferramentas de desenvolvimento instaladas com autorização, sem alterar go.mod:
  `goimports` de `golang.org/x/tools v0.52.0` e `golangci-lint v2.15.0`, compilados
  com o Go existente. Acrescente `$(go env GOPATH)/bin` ao PATH para usá-las.
- Referência diferencial: Hurl `8.0.0`, libcurl `8.18.0` já disponível na sandbox.
- GCC e headers C agora estão acessíveis na sandbox, inclusive `/usr/include`
  e `/usr/libexec/gcc/x86_64-redhat-linux/16/cc1`. O bloqueio inicial de
  visibilidade dessas dependências foi resolvido.

Em ambiente novo com ferramentas C disponíveis, os comandos de instalação das
ferramentas Go de desenvolvimento são:

```sh
go install golang.org/x/tools/cmd/goimports@v0.52.0
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0
```

A instalação dessas ferramentas não altera dependências do produto.

## Revalidação com race

Após disponibilizar os pré-requisitos C, a validação foi repetida com
`CGO_ENABLED=1`, `CC=gcc` e as ferramentas Go no PATH:

- `make fmt lint test`: passou integralmente, incluindo `go test ./... -race -cover`;
- `go vet ./...`: passou explicitamente;
- `make build compatibility portable`: passou, incluindo os nove cenários Hurl.

O utilitário `tools/dummy/main.go` é exclusivamente para uso manual em
desenvolvimento (`make serve` ou `make dummy`), documentado em `dev-server.md`.
Ele é excluído por build tag do build, lint e testes automáticos; não há testes
do dummy integrados à suíte, nem dependência de um serviço na porta 8080.
Na verificação manual, os dois cenários existentes passaram contra ele, e
Ctrl+C encerrou o processo e liberou a porta. O servidor temporário anterior
foi encerrado; o utilitário não ficou em background.

## Saída incremental

Os resultados agora são emitidos durante a execução: cada linha é escrita antes
do próximo GET, e o resumo aparece somente ao final, sem duplicar resultados.
As categorias e os exit codes permanecem inalterados. Nas linhas de sucesso,
inclusive sem expectativa, o prefixo de arquivo/linha/coluna foi removido: elas
começam diretamente com `GET`. Divergências e erros preservam a localização;
o resumo continua identificando o arquivo.

- Runner: callback síncrono testado em validação aprovada, ausência de expectativa,
  divergência, falha de transporte e erro de apresentação. Falhar ao apresentar um
  resultado impede todas as chamadas posteriores e preserva causa/posição.
- CLI: quarenta entradas comprovam resultado visível antes da próxima execução,
  sanitização e resumo único; falha no primeiro resultado, posterior ou resumo
  produz exit 2, sem expor erros brutos. Testes também verificam ausência de
  localização em sucessos e sua preservação nas falhas.
- E2E: binário real escrevendo em pipe; o servidor bloqueia a segunda requisição
  por canal até o teste observar a primeira linha no stdout. Só depois a resposta
  é liberada, em variantes de sucesso, divergência e body incompleto. Não há sleep
  nem dependência de tempo de resposta para sincronização.

## Evidências iniciais nesta sandbox (sem CGO)

Antes da correção do ambiente, com `CGO_ENABLED=0` e as ferramentas Go no PATH:

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

## Bloqueio inicial com race — resolvido

O ambiente inicialmente não expunha `cc1` nem os headers C, embora os pacotes
constassem instalados. A tentativa sem CGO também falhava, porque `-race` exige
CGO. Esse bloqueio foi resolvido ao disponibilizar os arquivos na sandbox, e o
gate completo passou na revalidação registrada acima.

`make test` continua executando `go test ./... -race -cover`, sem fallback,
remoção de verificações ou transferência da exigência para outra história.
Os resultados iniciais sem CGO foram preservados como histórico, não como
substituto da validação com race. Nenhuma task foi marcada `done` com base
somente na validação parcial.

Não é necessário CGO para o binário httpgo; o compilador C é necessário para o
race detector do ambiente de testes.

## Portabilidade: build versus runtime

| Plataforma/arquitetura | Build CGO=0 | Execução nativa E2E | Ctrl+C real | TTY real |
|---|---|---|---|---|
| Linux amd64 | passou | passou, suíte também validada com race | passou | passou via PTY |
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
conclusão na issue. O bloqueio local com race foi resolvido; ainda faltam as
validações nativas de macOS/Windows antes do aceite nessas plataformas.
