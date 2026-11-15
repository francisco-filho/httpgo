# Validação — US-01 a US-11 e US-20

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
