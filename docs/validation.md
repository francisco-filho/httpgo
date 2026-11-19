# Validação — US-01 a US-17, US-20 a US-41

## US-41 — issue #88 / história 041

Plano backend da issue #88 executado sem planner, em quatro tasks autoaprovadas
na fila `041`. Ciclo síncrono de arquivo: `6faff9a`; apresentador protegido:
`b479ab2`; seleção por TTY e resumo: `33fd7c5`; integração PTY e documentação:
esta alteração. Escopo somente backend, sem frontend/estáticos, nova linguagem,
mudança de execução, toolchain, CI/lint ou corpus de compatibilidade.

**Mudança deliberada de contrato:** formato texto em stdout e stderr TTY passa
a usar progresso compacto em stderr e resumo por arquivos tentados em stdout.
Redirecionar qualquer stream preserva integralmente o texto anterior. JUnit,
HTML, ajuda/versão e códigos 0/1/2 não mudam. `NO_COLOR` definido, mesmo vazio,
retira cores sem retirar atualizações da linha. A dependência portátil
`golang.org/x/term v0.48.0` (e `x/sys v0.50.0` indireta) foi adicionada só para
identificação de TTY real; Go permanece `1.26.10`.

### Evidência executada em Linux

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports` e golangci-lint existentes
no PATH. O cache Go padrão estava somente leitura. Para baixar/verificar a nova
dependência e executar os gates, foram usados `GOPATH=/tmp/httpgo-041-go` e
`GOMODCACHE=/tmp/httpgo-041-modcache`, sem instalar ferramentas ou trocar o Go.
Hurl **8.0.0**, já disponível em `/usr/local/bin/hurl`, foi usado sem instalação,
consulta/cópia/adaptação de código-fonte ou atualização de goldens. Seu aviso de
metadados libcurl não impediu o gate diferencial.

Passaram efetivamente:

- Em cada task: testes específicos, `make fmt lint test`, `go vet ./...` e
  `go build ./...`. Lint: **0 issues**; `make test` usa **-race -cover**.
- Task 1: `go test ./internal/suite ./internal/runner`; callbacks antes de envio,
  parsing completo, propagação de falha e arquivos pulados não carregados.
- Task 2: `go test ./internal/cli`; streams injetados e writers com erro/short
  write nas fronteiras de carga, total, avanço, estado final e diagnóstico.
- Task 3: `go mod tidy` e `go test ./internal/cli ./internal/report ./internal/suite`;
  seleção sobre streams originais, presença/ausência de NO_COLOR, resumo com
  duração/média determinísticas, zero tentados e regressões de relatórios.
- Task 4: `go test ./tests/e2e -count=1`,
  `go test ./internal/cli ./internal/suite ./internal/runner ./internal/report`,
  `make fmt lint test`, `go vet ./...`, `go build ./...`, `make portable` e
  `make compatibility`: **476 pares** com Hurl **8.0.0**, sem skip/atualização.
- `go test ./tests/e2e -run '^TestCompactRealTTY' -race -count=10`: dez execuções
  dos novos testes PTY, sem sleeps, falhas ou data races. Cada stream usa PTY ou
  pipe independente; canais mantêm HTTP pendente enquanto o harness observa
  nome, arquivo anterior finalizado e `0/N`, antes de liberar a resposta.
- Fuzzers separados, 20 segundos e dois workers por alvo, sem falhas:
  `FuzzParse` (**172.770** execuções), `FuzzRender` (**42.976**) e
  `FuzzRenderJSON` (**21.565**). Comandos:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`,
  e equivalentes em `./internal/template` para os outros dois alvos.

### Rastreabilidade GO-524–GO-538

| Cenário | Evidência automatizada |
|---|---|
| GO-524 | `TestFileLifecycleNotifications`, `TestCompactProgressLifecycle` e `TestCompactRealTTYPendingAndOutcomes`: `0/N` antes do envio, requisição real pendente com barreira, arquivo anterior visível |
| GO-525 | CLI de ciclo e PTY de desfechos: nomes-base iguais em diretórios distintos, sem expectativa e `HTTP *`, linhas finais próprias, sucesso e ordem preservados |
| GO-526 | `TestCompactFourthStepAndProtection` e PTY `fourth failure`: erro no quarto step, ordinal/posição e nenhum step posterior; CLI de ciclo verifica diagnóstico antes do próximo envio |
| GO-527 | suíte de notificações, CLI `retry` e PTY `retry pass`/`retry exhausted`: avanço único, tentativas/envios preservados |
| GO-528 | `TestDiscoveryCancellationAndLateLoadFailure`, CLI `read`/`parse` e `TestCompactRealTTYLoadErrorsAndProtection`: carga/parsing com total desconhecido e contagem de tentados |
| GO-529 | notificações e CLI com fail-fast; PTY de falha/esgotamento identifica não alcançados com `?`, sem carga nem envio |
| GO-530 | `TestCompactSummaryCountsAndDuration`, ciclo CLI e suíte mista real em PTY: tentados versus envios, erros agrupados, média, duração e não executados condicionais |
| GO-531 | `TestCompactSelectionAndColors` e `TestCompactRealTTYSelection`: oito combinações reais PTY/pipe e NO_COLOR, inclusive redirecionamentos parciais; texto/streams legados preservados |
| GO-532 | testes CLI e `TestCompactRealTTYReportsAndStaticCommands`: sem compacto em JUnit/HTML mesmo com ambos TTY, XML lido e contagem verificada; serializadores e E2E JUnit/HTML existentes também passam |
| GO-533 | seleção CLI/PTy com NO_COLOR ausente, vazio ou definido; ajuda/versão estáticas sem ANSI; rótulos mantidos com e sem cor |
| GO-534 | `TestCompactFourthStepAndProtection`, `TestCompactRenewalProtectionSurvivesFileChange` e PTY de segredos: nomes/diagnósticos protegidos, renovação/derivação e valores operacionais intactos |
| GO-535 | notificações, CLI `pre-send` e suíte mista real em PTY: erro pré-envio avança contador sem incrementar envio |
| GO-536 | `TestCancelledBeforeLoadingDoesNotNotify`, testes existentes entre arquivos, CLI `cancel`, resumo determinístico com zero tentados e PTY com SIGINT durante HTTP: código 2, desfecho alcançado e demais não executados |
| GO-537 | `TestCompactDoesNotAffectReportsHelpOrPreexecutionErrors`: uso/configuração/descoberta sem linhas ou resumo compacto; regressões globais existentes preservadas |
| GO-538 | `TestFileLifecycleNotifications`, `TestCompactProgressWriterFailures`, `TestCompactSummaryWriterFailure`: erro/short write em cada fronteira, código 2, sem novos envios, resumo indevido ou repetição no writer falho |

O workflow `.github/workflows/junit.yml` e scripts consumidores/contagens não
foram alterados. Não houve disparo remoto do CI ou análise SonarQube pelo backdev.
PTY e HTTP são evidência de execução **Linux** com binário real e servidor local,
não de API externa, execução nativa macOS/Windows ou publicação de checks no
GitHub. `make portable` comprovou somente cross-build Linux/Darwin/Windows amd64.
Proteção incremental não apaga texto já exibido antes da descoberta de segredo,
nem detecta todo segredo arbitrário não marcado ou transformação desconhecida.

## US-40 — correção da origem de distribuição (v0.1.2)

A origem pública correta é `francisco-filho/httpgo`. Instalador, comandos do
README, condição de publicação e smoke HTTPS do workflow foram alinhados a esse
repositório. Os testes de instalação e documentação exigem a mesma origem, e o
teste do workflow verifica tanto a restrição de publicação quanto a URL do smoke.
O módulo Go `capimgrosso.com/httpgo` permanece inalterado: ele não define a URL
dos downloads.

Antes da correção, a API pública de releases de `francisco-filho/httpgo` retornou
uma lista vazia, e o download de `latest/download/install.sh` retornou 404. Na
execução [44515147598](https://github.com/francisco-filho/httpgo/actions/runs/44515147598),
`package` e os dois testes nativos Linux passaram, mas `publish` e `public-smoke`
foram ignorados. A condição anterior exigia `capimgrosso/httpgo`, impedindo a
publicação no repositório real.

Validação local da correção: `make fmt lint test` passou (lint sem issues e testes
com `-race -cover`), assim como `go vet ./...`, `make build e2e` e
`bash -n install.sh scripts/package-release.sh`. A tag `v0.1.2` foi classificada
como estável por `scripts/package-release.sh --validate-tag`. Os downloads dos
testes permanecem simulados; esses resultados não comprovam publicação pública.

A correção local não publica assets: é necessário enviar a tag `v0.1.2` e aguardar
o workflow concluir antes de usar as URLs públicas. A matriz atual mantém apenas
Linux; os pacotes Darwin continuam sendo produzidos, mas sem validação nativa
macOS. O registro abaixo descreve a entrega original e suas pendências históricas,
incluindo a origem incorreta então preservada; não é evidência de publicação da
`v0.1.2`.

## US-40 — issue #82 / história 040

Plano backend da issue #82 executado sem planner. As quatro tasks foram
registradas e autoaprovadas na fila `040`: empacotamento (`0bec2c2`), instalador
(`fa579ac`), workflow (`d69720d`) e documentação/testes integrados (esta alteração).
A implementação local está disponível, mas **a Task 3 e o aceite de distribuição
pública não estão concluídos**: ainda faltam as evidências externas abaixo.
A documentação foi finalizada após o pedido do usuário, sem converter essas
pendências em sucesso nem modificar o plano ou os critérios de aceite.

`scripts/package-release.sh` compila Linux/Darwin amd64/arm64, com CGO desligado,
metadados por linkedição, um executável por pacote e quatro hashes SHA-256.
`install.sh` fixa uma tag, valida HTTPS/checksum/conteúdo do arquivo e prepara a
troca por renomeação no diretório de destino. O README apresenta instalação
padrão, versão explícita, diretório com espaços e inspeção do script; preserva a
alternativa por fonte, inclusive Windows, e avisa sobre código remoto e limites
do checksum. Os comandos públicos são condicionados à existência da release,
sem alegar publicação ou homologação já realizadas.

O workflow novo `.github/workflows/release.yml` encadeia gates Linux, os mesmos
artefatos em quatro runners nativos, uploads em rascunho, publicação e smoke
HTTPS público. A escrita fica restrita ao publicador e a publicação a pushes de
tag no repositório `capimgrosso/httpgo`. PRs não criam tags ou releases.
O workflow JUnit e `make portable` continuam independentes. Não foram alterados
parser, runner, transporte, contratos da CLI, Go, dependências, corpus de
compatibilidade ou frontend/estáticos.

### Evidência executada em Linux

Linux amd64, Go `go1.26.10-X:nodwarf5`; ferramentas já presentes no PATH.

- `bash -n install.sh scripts/package-release.sh` passou. Os blocos Bash do
  workflow também passaram em `bash -n`, pelo `TestReleaseWorkflowGuards`.
- O YAML do workflow foi lido com sucesso pelo parser `go.yaml.in/yaml/v3` já
  existente no módulo, em um programa temporário fora do repositório. Isso é
  **validação de sintaxe YAML, não execução do GitHub Actions nem actionlint**.
- `go test ./tests/release -count=1` e
  `go test ./tests/e2e -run '^(TestReadme|TestMarkdownAnchors)' -count=1` passaram.
  Os testes do README executam os blocos publicados, incluindo pipe, versão
  fixa, caminho com espaços e script baixado para inspeção, com um binário Go
  nativo real e substituto de curl isolado no PATH do harness. Não fazem
  download público nem comprovam inspeção humana.
- `make fmt lint test` passou: lint com **0 issues**, testes com **-race -cover**.
  `go vet ./...`, `go build ./...`, `make build` e `make e2e` passaram.
  `gofmt -l` nos arquivos Go alterados e `git diff --check` sem problemas.
- Empacotamento local com a versão sintética `v2.3.4`, sem criar tag Git:
  `bash scripts/package-release.sh v2.3.4 <temporário>/assets`, seguido de
  `HTTPGO_RELEASE_ASSETS=<temporário>/assets HTTPGO_RELEASE_TAG=v2.3.4 go test ./tests/release -run '^TestInstallerNativePackage$' -count=1 -v`.
  O teste instalou **os bytes do pacote produzido**, sem recompilá-lo ou
  recomprimi-lo, conferiu SHA-256 e iniciou `httpgo --version` em Linux amd64.
- `sha256sum` foi executado de fato. Este Linux não tem `shasum`: o fallback foi
  testado com um substituto que valida `-a 256` e encaminha o cálculo ao SHA-256
  real. A execução de `shasum` real e do Bash 3.2 permanece para os runners
  macOS. A presença dos testes na matriz **não comprova que ela já executou**.

### Rastreabilidade dos cenários

| Cenário | Evidência local / limite |
|---|---|
| GO-510 | `TestPackageRelease`: quatro nomes, membro regular executável único, SHA-256, CGO desligado, OS/arch e metadados; `--version` somente no alvo nativo. Publicação não executada |
| GO-511 | `TestReleaseTags` classifica pré-release; `TestInstallerSelection` exercita latest estável e pré-release explícita simulados. Classificação pública do GitHub ainda não comprovada |
| GO-512 | `TestInstallerNativePackage` instala pacote nativo Linux amd64; `TestReadmeInstallCommands` inicia binário nativo pelos comandos publicados. Faltam os outros três ambientes e o smoke público dos quatro |
| GO-513 | `TestInstallerSelection` e `TestInstallerFailuresPreserveExecutable`: aliases e plataformas recusadas, com uname substituído no harness |
| GO-514 | `TestInstallerSelection`, `TestInstallerFailuresPreserveExecutable`: verificadores, hashes adulterados, ausentes, duplicados ou divergentes; shasum real pendente conforme nota acima |
| GO-515 | `TestInstallerFailuresPreserveExecutable`: tag inexistente e pacote ausente, sem fallback nem alteração do binário anterior |
| GO-516 | `TestInstallerSelection`, `TestInstallerPathAlreadyConfigured`, `TestReadmeInstallCommands`: criação, reinstalação, destinos padrão/relativo/com espaços, mensagens e PATH; sem sudo nem alteração dos arquivos de shell |
| GO-517 | `TestReadmeInstallContract`, `TestReadmeInstallCommands`, `TestReadmeLinksAndProgression`: pré-requisitos, limites, comandos publicados, execução nativa local e links/flags de cada componente |
| GO-518 | `TestReleaseTags`, `TestPackagePreparationFailures`, `TestReleaseWorkflowGuards`: SemVer inválido, falhas de build/tar/hash, ordem e gates do workflow. Falha real de upload no GitHub não foi provocada |
| GO-519 | `TestInstallerFailuresPreserveExecutable`: curl simula erro de rede/TLS com download parcial; argumentos HTTPS/redirecionamentos são verificados, temporários limpos e instalação anterior preservada. Não é handshake TLS público |
| GO-520 | `TestInstallerFailuresPreserveExecutable`, `TestInstallerRejectsExtraMembers`: gzip inválido, arquivo ausente/vazio, caminho inseguro, links, membros extras e erro na extração |
| GO-521 | `TestInstallerIOFailures`: falhas injetadas na criação, preparação, cópia, chmod e renomeação, inclusive sob root, sem truncar ou remover os bytes/permissões anteriores |
| GO-522 | `TestInstallerLatestPinnedWhenReleaseChanges`: o próximo redirecionamento simulado aponta para outra estável, mas pacote e checksum permanecem na primeira tag, com uma única resolução de latest |
| GO-523 | `TestInstallerPrerequisites`, `TestInstallerFailuresPreserveExecutable`, `TestInstallerHelp`: ferramentas ausentes, opções inválidas/sem valor, fallback SHA e ajuda, antes de alterar o destino |

### Pendências impeditivas para concluir a distribuição pública

- Não houve push, criação de tag, publicação de release nem execução do workflow
  remoto nesta sessão. A consulta HTTPS de `capimgrosso/httpgo/releases/latest`
  durante a implementação retornou **404**; os comandos no README não são
  evidência de assets já disponíveis.
- O `origin` deste checkout é `francisco-filho/httpgo`, enquanto o plano fixa a
  origem pública em `capimgrosso/httpgo`. Essa decisão foi preservada; nenhum
  remote foi reconfigurado e nenhuma publicação foi presumida nesse outro
  repositório.
- Faltam execuções reais dos pacotes nos quatro runners da matriz e instalações
  públicas HTTPS, por tag e, para estáveis, latest. Também não foi comprovado no
  serviço o fluxo estável → pré-release nem o comportamento de upload incompleto.
  **CA-02 não está completo**; cross-build, inspeção dos metadados, simulação ou
  parsing YAML não substituem essas evidências.
- A fila mantém a Task 3 não concluída e a história não foi enviada ao review
  final. Não há comentário GitHub de conclusão da história nem análise SonarQube
  pelo backdev. A entrega não conclui a US-19 ou amplia seu escopo de portabilidade.

## US-39 — issue #81 / história 0039

Plano da issue #81 executado sem planner, com três tasks backend registradas e
autoaprovadas na fila `0039`. Task 1: `f36449c` (configuração de ambientes e
overrides); task 2: `836d53c` (captures); task 3: esta alteração (assertions,
integração das referências e validação final).

Publicados `configuration.md`, `captures.md` e `assertions.md`, com links e
introduções breves no README. Exemplos e receitas dos três temas saíram do
README para os guias; os testes continuam lendo os conteúdos publicados, sem
substituí-los por cópias independentes. Subset/arquitetura foram alinhados à ordem
e renovação vigentes da US-34. A ajuda esclarece que ausência na comparação `!=`
isolada é divergência 1, mas ausência na captura `new_token` do cenário completo
é erro 2 anterior à assertion. Nenhuma funcionalidade, sintaxe, fonte de
configuração ou contrato público foi ampliado. Sem frontend/estáticos, alterações
de dependências, toolchain, CI/lint ou corpus `testdata/compatibility/`.

### Evidência executada em Linux

Go `go1.26.10-X:nodwarf5`, linux/amd64, ferramentas existentes no PATH.

- Em cada task: `make fmt lint test`, `go vet ./...` e `make build` passaram;
  lint com **0 issues**, testes com **-race -cover**.
- Task 1: testes específicos `TestConfigurationGuidePrecedence`,
  `TestGuideLinksAndOptions`, `TestReadme*` e `TestMarkdownAnchors`. Três servidores
  `httptest` distintos comprovam destinos base/ambiente/CLI a partir dos YAML
  publicados; conflitos e duplicação CLI retornam 2 sem novos envios.
  O teste de credenciais lê o YAML/cenário do guia e comprova referência ausente
  obrigatória antes de HTTP, sem vazar o token fictício.
- Task 2: `TestCapturesGuideCount` lê o cenário do guia, comprova 0/1/2,
  preservação da quantidade mesmo mudando conteúdo, array vazio, captura
  ausente/null/escalar como 2, ausência na assertion como 1 e falta do opt-in
  como rejeição antes de HTTP. Testes existentes de header, regex, projeção e
  filtros estáticos/dinâmicos verificam conteúdo literal das fixtures no guia.
- Task 3: `go test ./internal/cli -count=1` e testes específicos dos guias,
  README, links, âncoras, exemplos publicados e documentação passaram.
  `TestAssertionsGuideCatalog` extrai e executa **32 declarações das tabelas**
  contra HTTP local real: 20 JSONPath, 5 header, 2 body e 5 extração regex.
  `TestAssertionsGuideFailureDistinctions` cobre tipos, ausência/null, notação
  inteira, percurso estrito, perfil literal/opt-in, regex e formas não suportadas.
  `TestAssertionsGuideRotation` lê o cenário publicado e comprova aprovação,
  repetição como 1 e captura ausente como 2, sem GET posterior nem tokens na saída.
- Gate integrado: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build` e `make e2e` passaram. `gofmt -l` nos arquivos Go alterados e
  `git diff --check` sem problemas. Builds emitiram aviso não fatal de cache de
  metadados do módulo em filesystem somente leitura; terminaram com sucesso.
- Fuzzing, 20 segundos e dois workers por alvo, sem falhas:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**23.348** execuções),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**20.750** execuções) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**34.034** execuções).

O diferencial Hurl não foi reexecutado nem regenerado nesta história de
documentação. Não houve análise SonarQube pelo backdev. Evidência de binário real
com servidores locais e portas dinâmicas em Linux; não de APIs externas nem
execução nativa macOS/Windows. A validação de portabilidade permanece para release.

## US-38 — issue #79 / história 0038

Plano backend da issue #79 executado sem planner, com três tasks registradas e
autoaprovadas na fila `0038`. Task 1: `97c695a` (descoberta e execução das duas
extensões); task 2: `89fb3de` (ajuda, documentação e exemplos publicados);
task 3: esta alteração (revisão da integração e evidência dos gates).

A alteração de produção está restrita à seleção e ao diagnóstico de elegibilidade
em `internal/suite/discovery.go`, além da ajuda da CLI. Parser, runner, transporte
e relatórios usam o mesmo fluxo, sem modo por extensão. Corpus diferencial,
budgets, Go, dependências, CI e lint permanecem inalterados. Não houve trabalho
em frontend/estáticos nem análise SonarQube pelo backdev.

### Evidência executada em Linux

Linux amd64, Go `go1.26.10-X:nodwarf5`. `goimports` e `golangci-lint` existentes
em `/home/ff/go/bin`, acrescentado ao PATH, sem instalação dessas ferramentas.
O Hurl padrão era **8.0.1**, incompatível com o gate fixado. Após autorização
explícita, foi baixado o binário oficial **8.0.0**, sem consultar código-fonte:
[arquivo da release](https://github.com/Orange-OpenSource/hurl/releases/download/8.0.0/hurl-8.0.0-x86_64-unknown-linux-gnu.tar.gz).
SHA-256 conferido contra o digest publicado na API da release:
`6ab1d6703b94086ece6d2f994d73f8d215926997b292ba29b894799425afbf67`.
Extração temporária em `/tmp/httpgo-0038-hurl`, usada somente via PATH; nenhuma
substituição da instalação do usuário. `hurl --version` confirmou **8.0.0** com
libcurl **8.18.0**. O aviso de metadados libcurl não impediu a execução.

Passaram efetivamente:

- Task 1: `go test ./internal/suite ./internal/cli -count=1` e
  `go test ./tests/e2e -count=1`.
- Task 2: `go test ./internal/cli -count=1`,
  `go test ./tests/e2e -run '^(TestReadme|TestMarkdownAnchors)' -count=1` e
  `go test ./tests/e2e -count=1`. Expectativas antigas da ajuda e marcadores dos
  exemplos foram ajustados sem remover verificações de conteúdo/execução.
- Em cada task de implementação: `make fmt lint test`, `go vet ./...` e
  `go build ./...`, com lint **0 issues** e testes **-race -cover**.
- Gate integrado: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build`, `make e2e` e `make compatibility`. `gofmt -l` em todos os
  arquivos Go alterados e `git diff --check` não apontaram problemas. O
  diferencial executou os **476 pares**
  existentes com Hurl **8.0.0**, sem skip, atualização ou regeneração de goldens.
- Fuzzing separado, 20 segundos e dois workers por alvo, sem falhas:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**131.057** execuções);
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**24.459** execuções);
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**7.845** execuções).

Cobertura observada no gate: `suite` **95,1%**, `cli` **98,4%**, `parser`
**97,1%**, `runner` **95,3%**. Cobertura não foi usada para reduzir verificações.

### Rastreabilidade dos cenários

| Cenário | Evidência automatizada executada |
|---|---|
| GO-501 | `TestScenarioExtensionsDemo`: binário real e `httptest.Server`, conteúdo extraído da seção de compatibilidade do README, arquivos individuais e diretório misto, ordem, resumos, stdout/stderr e código 0; conteúdo idêntico não elimina caminhos distintos |
| GO-502 | `TestDiscoverScenarioExtensions` e `TestScenarioOnlyHTTPGoRecursiveAndExplicitExtensions`: diretório somente com extensão recomendada, inclusive subdiretório |
| GO-503 | `TestDiscoverNormalizedUniqueOrder`, `TestDiscoverEquivalenceUsesFilesystem`, `TestDiscoverScenarioExtensions` e `TestSuiteDiscoveryRealHTTP`: sobreposições/argumentos invertidos, hard links entre extensões, mesmo nome-base em cópias independentes e ordem global; caso com legado antes da extensão recomendada impede preferência por extensão |
| GO-504 | `TestScenarioExtensionEquivalentValidation`: mesmos conteúdos sob ambas as extensões; sucesso 0, header divergente 1, erro estrutural tardio e HEAD não suportado 2, nenhum envio do arquivo rejeitado e resumo de pré-validação |
| GO-505 | `TestSuiteDiscoveryAndConfiguration`, `TestSuiteDiscoveryRealHTTP` e `TestScenarioOnlyHTTPGoRecursiveAndExplicitExtensions`: auxiliares ignorados e cenários explícitos com outra extensão ou sem extensão executados |
| GO-506 | `TestReadmeScenarioExtensionsAndPublishedPaths`, `TestReadmePublishedScenarioCopies`, `TestReadmeLinksAndProgression`, `TestMarkdownAnchors`, testes existentes de exemplos e ajuda: exemplos padrão, restrição dos legados à compatibilidade, limites explícitos, caminhos reais e preservação dos conteúdos exercitados |
| GO-507 | `TestDiscoverErrorsDiscardWholePlan` e `TestSuiteDiscoveryAndConfiguration`: diretórios sem elegíveis, inclusive só auxiliares, rejeitam todo o plano; diagnóstico cita ambas as extensões e executor não é chamado |
| GO-508 | `TestDiscoverLinks`, `TestDiscoverErrorsDiscardWholePlan`, `TestSuiteDiscoveryAndConfiguration` e `TestSuiteDeniedAccessUnprivilegedProcess`: links com extensão nova ignorados/rejeitados conforme origem; falhas injetadas e acesso real negado por processo não privilegiado, sem HTTP |
| GO-509 | `TestSuiteRealIsolationProtectionAndPayloadOrigins`: suíte mista com cenário da extensão nova, cwd independente e configuração externa; servidor confere bytes JSON e multipart relativos ao cenário, valores iniciais, ausência de cookie implícito e captura não resolvível no arquivo seguinte |

Regressões GO-217/GO-219 da US-16 foram reutilizadas na suíte mista, não
substituídas por um catálogo paralelo. O teste E2E de isolamento confere também
proteção de captura secreta, cookie, derivados, nomes de arquivo, assertion e
erro estrutural posteriores, sem alterar dados operacionais enviados. As demais
regressões de continuação, cancelamento e falhas de escrita permanecem no gate.

Os exemplos recomendados foram acrescentados sem renomear cenários antigos;
`TestReadmePublishedScenarioCopies` verifica que seus bytes permanecem iguais
às fixtures legadas exercitadas. Os exemplos de suíte em `testdata/examples/`
evitaram duplicar cenários nos diretórios legados já usados pelos testes.

Esta evidência é de testes HTTP locais e binário real em Linux, não homologação
de APIs externas ou execução nativa macOS/Windows. A demonstração usa porta
dinâmica em `httptest`, não depende de serviço externo ou porta 8080 livre.
Validação nativa de portabilidade permanece para release conforme o plano v4.

## US-37 — issue #77 / história 0037b

Plano da issue #77 executado sem planner, com cinco tasks registradas e
aprovadas na fila `0037b`. Task 1: `c3ba921` (instalação e primeira execução);
task 2: `3e5fe31` (requisições, dados, configuração, credenciais e relatórios);
task 3: `cccd7fb` (guia de assertions/captures e testes dos blocos publicados);
task 4: `a4ff2fb` (extensões e limites frente ao Hurl); task 5: esta alteração
(consolidação progressiva, links e reprodução dos exemplos).

Escopo somente documentação e testes E2E do backend. Sem alteração de produção,
contrato da CLI, linguagem, corpus `testdata/compatibility`, frontend/estáticos,
dependências, toolchain, lint ou CI. O README deixa de ser um histórico por
história; as receitas avançadas preservadas ficam opcionais e recolhíveis.

### Evidência executada em Linux

Go `go1.26.10-X:nodwarf5`, `goimports` e golangci-lint `2.15.0` já disponíveis
em `/home/ff/go/bin` (adicionado ao PATH, sem instalar ferramentas).

- Em cada task: `make fmt lint test` e `go vet ./...` passaram. Lint com
  **0 issues**, testes com **-race -cover**, sem relaxar verificações.
- Gate final: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build` e `make e2e` passaram. `gofmt -l` nos três arquivos Go novos e
  `git diff --check` sem diferenças de formatação/whitespace.
- `go test ./tests/e2e -run '^TestReadme' -count=1`: extrai os blocos do README,
  executa o binário real e usa servidores `httptest` com portas dinâmicas.
  Confere requests, dados tipados, headers, arquivos relativos, partes MIME,
  continuidade, capturas, códigos de saída, stdout/stderr e artefatos.
- Reprodução manual separada na porta 8080 com
  `go run ./tools/dummy/main.go`: conteúdo copiado dos blocos do README para uma
  pasta temporária fora do clone. Primeiro cenário e alteração do esperado
  retornaram **0/1**; JSON/header, body externo, variáveis tipadas, assertions de
  header/JSON/texto e encadeamento simples/dinâmico retornaram **0**. Divergência
  JSON e reprovação antes de publicar captura retornaram **1**. Configuração,
  ambiente e override retornaram **0**. Credencial fictícia injetada no ambiente
  retornou **0** sem revelar valor; referência ausente retornou **2**. Nenhum
  segredo real foi usado e o processo dummy foi encerrado após a verificação.
- Na mesma reprodução, suíte com dois arquivos retornou **0** em texto/JUnit/HTML;
  XML com dois testcases aprovados e HTML timestampado encontrados em `target`,
  com stdout vazio nos formatos de arquivo. Renderização externa opt-in
  retornou **0**. O modo literal sem flag é coberto por
  `TestReadmeExternalRendering`, que confere bytes enviados e divergência **1**
  contra a fixture que rejeita o template não resolvido com 400.
- Upload multipart do guia não usa o dummy: `TestReadmeExternalBodyAndMultipart`
  fornece a API com o contrato explicitado e confere nomes, textos, bytes,
  filename, MIME e status 201. Receitas opcionais usam as fixtures e testes
  existentes de cache, regex/cookies, inclusão/ausência, tipos, projeção,
  filtros, esperados dinâmicos, contagem e desigualdade. Os testes que exigem
  correspondência exata entre essas fontes e README continuam passando,
  sem alterar nem remover verificações anteriores.
- `TestReadmeLinksAndProgression` confere ordem das seções, caminhos/âncoras locais
  e opções dos comandos contra a ajuda atual. Links públicos Hurl
  (`/`, `docs/templates.html`, `docs/manual.html` e sua seção `configuration`)
  acessados via HTTPS com sucesso. A documentação pública foi consultada para
  distinguir contratos próprios de capacidades também existentes no Hurl;
  nenhum código-fonte Hurl foi consultado/copiado/adaptado.
- Fuzzing, 20s e dois workers por alvo, sem falha:
  `FuzzParse` (**104.082** execuções), `FuzzRender` (**17.534**) e
  `FuzzRenderJSON` (**39.277**), em `internal/parser` e `internal/template`.
- Compilação Windows amd64 com `CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build`
  para destino temporário passou; **não** é evidência de execução nativa.

### Critérios de aceite e limites

CA-01: `TestReadmeFirstRunAndRequestData` e reprodução manual de sucesso/falha.
CA-02: testes de dados, multipart, configuração, credenciais, suíte e relatórios.
CA-03: `TestReadmeAssertionsAndCaptures` cobre 0/1/2, esperado dinâmico, captura
inválida e descarte sem GET seguinte. CA-04: seção separa extensão, diferença e
não suportado, com exemplo `--render-files` executado. CA-05: teste de links,
ajuda, códigos, credenciais fictícias protegidas e referências vigentes.

O gate diferencial Hurl não foi reexecutado nesta história de documentação;
a matriz existente continua sendo a evidência versionada de compatibilidade,
sem ampliar anúncios ou alterar goldens. Não houve homologação de APIs reais,
CI remoto, navegador, execução nativa macOS/Windows ou análise SonarQube.
Esta última continua exclusiva do agente QA.

## US-36 — issue #74 / história 0036

Plano backend da issue #74 executado sem planner, quatro tasks registradas e
autoaprovadas na fila 0036. `back:1`: `a49aa13` (variante parser/query/runner/CLI,
cinco predicados e demonstração); `back:2`: `39f01f8` (perfis, transações, polling,
recursos e isolamento); `back:3`: `2a29aeb` (sigilo, callbacks, texto/JUnit e falhas
de escrita); `back:4`: esta alteração (ajuda, documentação, corpus e gates).
Base `2c0ee3d`. Sem frontend/estáticos, mudança de transporte, novas dependências,
Go/toolchain, CI ou lint. Nenhuma análise SonarQube foi executada pelo backdev.

### Evidência executada em Linux

Linux amd64, Go `go1.26.10-X:nodwarf5`, ferramentas já existentes em
`/home/ff/go/bin`. Hurl **8.0.0** reutilizado de
`/tmp/httpgo-0034-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` via PATH; nenhuma
instalação, download ou substituição pela versão padrão 8.0.1. O aviso de
metadados libcurl não impediu identificação/execução. Sem consulta/cópia/adaptação
de código-fonte Hurl ou contato com API externa/Borapedir.

Passaram:

- Task 1: `go test ./internal/parser ./internal/query ./internal/assertion ./internal/runner ./internal/cli -count=1`
  e `go test ./tests/e2e -count=1`. Cinco predicados, presença/ausência/vazio,
  primeiro grupo/match, captura estrita, sem grupo como avaliação 2, pré-validação
  integral e demonstração com binário real/HTTP local, POST exato e sigilo básico.
- Task 2: `go test ./internal/runner ./internal/cli ./internal/suite ./internal/transport -count=1`
  e `go test ./tests/e2e -count=1`. Esperados literal/dinâmico/provisório,
  extração estática, não recursão, esperado inválido antes de decode/seleção,
  barreiras/primeira falha, retry só de divergência, reenvio com valores antigos,
  confirmação/descarte, cancelamento, charsets, redirects, resposta incompleta,
  leitura/close, timeout e isolamento, sem sleeps/API externa.
- Task 3: `go test ./internal/runner ./internal/cli ./internal/report ./internal/suite -count=1`
  e `go test ./tests/e2e -count=1`. Inspeção de resultados/callbacks e stdout,
  stderr/XML completos; segredos antigos, renovados e derivados também em lotes
  descartados; writers com erro/short write/close, prioridade 2 e sem replay.
- Em cada task, `make fmt lint test`, `go vet ./...` e `make build` passaram
  antes do commit. `make test` usa **-race -cover**; lint: **0 issues**.
- Gate integrado final: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build`, `make e2e` e `make compatibility` passaram. **476 pares**, sendo
  **28 novos `assert_regex_*`** e **448 históricos sem alterações**. Atualização
  seletiva somente dos novos goldens, revisados e reexecutados sem `-update`;
  o gate completo também sem atualização ou skips. **Mudança de contrato do corpus
  sinalizada:** nova query aceita, diferenças deliberadas de extração estática,
  falta de grupo, códigos, validação integral e transação do polling publicadas
  em [compatibility.md](compatibility.md#assertions-sobre-extração-regex--us-36--issue-74).
  O negativo `render_asserts_unsupported_body_regex` continua rejeitando `body matches`.
- Coberturas finais: parser **97,1%**, query **89,0%**, assertion **96,9%**,
  runner **95,3%**, CLI **98,4%**, suite **95,1%**, report **97,1%**,
  template **96,1%**, transport **90,4%**. Nenhum limiar foi relaxado.
- Seeds ampliados e fuzzing executado separadamente, 20s e 2 workers cada:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**55.452** execuções), `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**25.868**) e equivalente com `'^FuzzRenderJSON$'` (**11.992**).
  Todos sem crash/falha.
- Ajuda raiz/test e cenário publicado conferidos pelo binário em
  `TestRegexAssertionPublishedContract` e `TestRegexAssertionSignupByBinary`;
  negativos versionados também em LF/CRLF em `TestRegexAssertionStructuralCorpus`.
  Links locais da documentação alterada conferidos; `git diff --check` e
  `gofmt -l` nos Go alterados sem diferenças.

### Rastreabilidade GO-487–GO-500

| GO | Evidência |
|---|---|
| 487 | E2E `TestRegexAssertionSignupByBinary`: demonstração versionada, POST exato `123456`, códigos 0/1, sigilo e interrupção |
| 488–489 | query `TestSelectRegex`, runner `TestRegexAssertions`, `TestRegexAssertionDoesNotPublish` e CLI `TestRegexAssertionCLI`: cinco predicados, primeiro grupo/match, opcional ausente e vazio, sem variável implícita |
| 490 | parser `TestRegexAssertionValidation`, E2E `TestRegexAssertionPreflightByBinary` e corpus estrutural: inválidos tardios/escapes/exclusões, LF/CRLF, nenhuma rede |
| 491 | runner/CLI de semântica, `TestRegexTransactionalPolling`, diferencial `no_group` e `capture_first`: erro não negável/retentável, captura ausente continua estrita |
| 492–493 | runner `TestRegexExpectedProfiles`, `TestRegexTransactionalPolling`, E2E perfis/polling: literal/dinâmico, extração estática, captura atual, não recursão, esperado inválido antes de consulta/decode |
| 494–495 | runner `TestRegexTransactionalPolling`, `TestRegexCancelBeforeConfirmation`, E2E `TestRegexAssertionPollingByBinary`: barreiras, primeira falha, valores antigos nos retries, confirmação só da tentativa aprovada, métricas/callback terminal |
| 496 | 28 novos pares diferenciais e 476 totais com Hurl 8.0.0, ajuda/docs/exemplo publicado executados |
| 497 | transport `TestRegexAssertionFinalResponse`, `TestRegexAssertionReadAndClose`: body final, charsets/UTF-8, redirects, incomplete/read/close, timeout/cancelamento |
| 498 | runner `TestRegexAssertionSafeResults`, E2E `TestRegexAssertionSecretSuiteByBinary`: padrões/esperados/segredos/derivados ocultos, ausência versus divergência e failure/error JUnit |
| 499 | CLI `TestRegexAssertionWriterFailures`, `TestRegexAssertionJUnitWriterFailures`: erros/short writes/close após 0/1/2, sem replay/anúncio falso, prioridade 2 |
| 500 | runner de perfis/transações repetidas, suite `TestRegexAssertionFileIsolation`, CLI `TestRegexAssertionCommandReuse`, E2E suíte de segredos: planos imutáveis e isolamento entre tentativas/arquivos/invocações |

Limites: corpus sintético não homologa fluxos reais, engine ou formato Hurl
completos. Query `variable`, `body matches`, regex dinâmica de extração, pipelines,
escolha de grupo e regex de header como query de assertion continuam excluídos.
YAML hierárquico/interpolação externa são extensões HTTPGo. Não há detecção
universal de segredos arbitrários/transformações desconhecidas; stderr não pode
redigir retroativamente valores ainda desconhecidos. JUnit conserva schema e
granularidade por arquivo. Nenhuma execução nativa macOS/Windows ou CI remoto
foi realizada; portabilidade opcional nesta história conforme plano v4.

## US-35 — issue #72 / história 0035

Plano da issue #72 executado sem planner, somente backend, com quatro tasks
registradas e autoaprovadas na fila 0035. `back:1`: `51af3d2` (CLI/renderer HTML,
detalhes seguros e duração lógica); `back:2`: `ee62ac3` (destinos exclusivos,
colisões, permissões e falhas de I/O); `back:3`: `d4b1990` (sigilo, interrupções,
isolamento e equivalência de execução); `back:4`: esta alteração (ajuda,
contrato/exemplos e regressão integrada). Base `6ba52fd`. Sem frontend/estáticos,
nova dependência, atualização de Go, alteração de parser/transporte, CI, lint ou
corpus de compatibilidade. Não foi consultado/copiado/adaptado código-fonte Hurl.

### Evidência executada em Linux

Linux amd64, Go `go1.26.10-X:nodwarf5`, ferramentas Go existentes em
`/home/ff/go/bin`, golangci-lint `2.15.0`. Para o gate diferencial foi reutilizado
Hurl **8.0.0**, já disponível em
`/tmp/httpgo-0034-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, somente via PATH.
Nenhuma ferramenta foi instalada; Hurl padrão 8.0.1 não foi usado como substituto
do gate 8.0.0. Aviso de metadados libcurl não impediu a identificação/execução.

Passaram:

- Task 1: `go test ./internal/runner ./internal/report ./internal/cli` e
  `go test ./tests/e2e -count=1`, com binário real e HTTP local, códigos 0/1/2,
  fail-fast, erro de cenário, arquivo único, escaping e contagens desconhecidas.
- Task 2: `go test ./internal/cli ./internal/report` e E2E `TestHTML*`.
  Relógio controlado comprova UTC/milissegundos, captura única do instante,
  primeiro sufixo livre e bytes anteriores preservados. Falhas injetadas cobrem
  open/write/short write/close/confirmação e preservação de causas. Permissão
  negada real em processo Linux sem privilégios (`uid=1000`; teste também
  rebaixa credenciais quando executado como root), sem tomar só chmod como prova.
- Task 3: `go test ./internal/cli ./internal/suite ./internal/runner ./internal/report`
  e `go test ./tests/e2e -count=1`. Inspeção do HTML **inteiro**, stdout/stderr,
  requests, capturas/derivados e metadados tardios; cancelamento por SIGINT real,
  writers com falha e comparação texto/JUnit/HTML sem replay.
- Em cada task de código, `make fmt lint test`, `go vet ./...` e `go build ./...`
  passaram antes do commit. Lint: **0 issues**; testes com **-race -cover**.
- Gate integrado: `make build`, `make fmt lint test`, `go vet ./...`,
  `go build ./...`, `make e2e` e `make compatibility` passaram. **448 pares**
  históricos com Hurl **8.0.0**, sem skips, `-update` ou alteração dos goldens.
  Coberturas: CLI **98,3%**, report **97,1%**, runner **95,0%**, suite **95,1%**,
  template **96,1%**, parser **97,4%**, transport **90,4%**.
- Fuzzing separado, 20s e 2 workers em cada comando:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**14.308** execuções);
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**80.062** execuções) e equivalente para `'^FuzzRenderJSON$'`
  (**31.985** execuções). Todos sem crash/falha.
- Golden `testdata/report/html.html` gerado com `-update` e conferido depois sem
  atualização. **Conferência manual offline feita e confirmada pelo usuário**
  nesta sessão: aparência/conteúdo corretos, tema claro, tabelas e texto de script
  escapado, sem execução. O agente não executou navegador neste ambiente.
- Ajuda e demonstração versionada validadas pelo binário real em
  `TestHTMLDocumentedSuiteAndHelp`, com/sem fail-fast; `git diff --check` e
  `gofmt -l` dos arquivos Go alterados sem diferenças de formatação.

### Rastreabilidade GO-475–GO-486

| GO | Evidência |
|---|---|
| 475 | CLI `TestHTMLResultsAndKnownCounts`, E2E `TestHTMLRealDemonstration` e `TestHTMLDocumentedSuiteAndHelp`: arquivo único/suíte, 0/1/2, detalhes/status e caminho efetivo |
| 476 | CLI `TestHTMLPreexecutionErrorsDoNotOpenReportOrSend`, `TestHTMLArguments`; E2E `TestHTMLMissingDestinationPreventsRealHTTP`: erro global/uso sem rede nem artefato fictício |
| 477 | `TestHTMLTimestampCollisionAndPermissions`, `TestHTMLCreatesPrivateDestination`, E2E `TestHTMLRealRepeatedNamesPreserveArtifacts`: UTC, pontos/extensão, sufixos, pais e preservação |
| 478–479 | report `TestHTMLGoldenOfflineAndEscaping`, CLI de resultados e E2E de sigilo: golden, CSS claro embutido, sem recursos externos, aspas/Unicode/marcação como texto; confirmação offline pelo usuário |
| 480 | CLI `TestHTMLFinalProtectionCoversEarlierNamesURLsAndDiagnostics`, E2E `TestHTMLRealSecretsAreNotInSerializedDocument`: ambiente, captura/derivação, URL/query, esperado/real, headers e descoberta posterior, sem dumps |
| 481 | E2E `TestHTMLRealPollingCapturesAndIsolationMatchOtherFormats`: sequência idêntica texto/JUnit/HTML, polling/captura/isolamento, cinco envios reais contra quatro entradas lógicas iniciadas |
| 482 | CLI de resultados/interrupções e E2E de demonstração: não iniciadas conhecidas, pré-validação sem envios, skipped desconhecido, sem sucesso inventado |
| 483 | runner `TestLogicalEntryDurationIncludesPreparationRetriesAndWait`: relógio controlado inclui preparo/retry/espera, exclui callback; duração não negativa, um terminal e métricas preservadas |
| 484 | CLI `TestHTMLDestinationFailuresAndResourceClosure`, `TestHTMLExclusiveOpenCollisionAndJoinedCauses`, report `TestHTMLWriterFailures`, E2E `TestHTMLRealPermissionFailure`: falhas, causas, close e confirmação |
| 485 | CLI `TestHTMLInterruptionsKeepAvailableResultsWithoutRetryingStderr`, E2E `TestHTMLRealCancellationStillGeneratesAvailableResults`: sem próximo envio ou retry no stream, artefato disponível |
| 486 | CLI `TestHTMLCommandReuseDoesNotRetainDataOrDestinations` e teste de colisão: invocações isoladas, sem herdar capturas/proteção/detalhes/caminho final |

Limites: HTML próprio de apresentação, sem prometer flags equivalentes às do Hurl,
publicação externa, gravação transacional ou remoção de parcial. stderr não pode
redigir retroativamente um valor desconhecido; proteção não detecta todo segredo
arbitrário/transformação não marcada. Nenhuma homologação de API externa, CI remoto,
SonarQube ou execução nativa macOS/Windows foi realizada (estas plataformas são
opcionais na história). SonarQube continua exclusivo do agente QA.

## US-34 — issue #70 / história 0034

Plano da issue #70 executado sem planner, somente backend, com quatro tasks
registradas/autoaprovadas na fila 0034. `back:1`: `81db181` (sessão provisória,
ordem transacional e demonstração); `back:2`: `5af86f9` (polling, fontes/tipos,
cancelamento e isolamento); `back:3`: `9dc437f` (proteção monotônica,
callbacks/writers e suíte texto/JUnit); `back:4`: esta alteração (contrato,
compatibilidade, ajuda e fuzzing). Base `e18c519`. Sem frontend/estáticos,
novas dependências, alteração de Go, transporte, CI ou lint. Backdev não executou
SonarQube. Nenhum código-fonte Hurl foi consultado/copiado/adaptado.

### Ambiente e validações efetivamente executadas

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports`/`golangci-lint` existentes em
`/home/ff/go/bin`. O Hurl padrão era 8.0.1. Com autorização explícita do usuário,
o binário oficial **8.0.0** foi baixado para
`/tmp/httpgo-0034-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, sem alterar a
instalação existente. SHA-256 do arquivo tar conferido contra o asset oficial:
`6ab1d6703b94086ece6d2f994d73f8d215926997b292ba29b894799425afbf67`.
Aviso de metadados libcurl não impediu identificação nem execução de 8.0.0.

Passaram:

- Task 1: `go test ./internal/template ./internal/runner ./internal/cli ./internal/parser`
  e `go test ./tests/e2e -count=1`, incluindo o cenário publicado com servidor
  local real, seleção própria, renovação sucessiva e seções invertidas.
- Task 2: `go test ./internal/template ./internal/runner ./internal/suite ./internal/cli ./internal/transport`.
  Executor falso e espera controlada, sem sleeps/API externa, verificam valores
  anteriores em URL/body de reenvio, lote atual nas assertions, esgotamento,
  erro terminal de captura, cancelamento na espera e após Stage antes de confirmar.
- Task 3: `go test ./internal/template ./internal/runner ./internal/cli ./internal/suite ./internal/report`
  e `go test ./tests/e2e -count=1`. Inspecionados callbacks, metadados, stdout,
  stderr e XML, além de códigos/requests. Falhas/short writes não reenviam nem
  reimprimem; sobrescrita e descarte preservam valores secretos/derivados.
- Em **cada task**, `make fmt lint test`, `go vet ./...` e `go build ./...`
  passaram antes do commit. Lint **0 issues**; `make test` usa **-race -cover**.
- Gate integrado da task 4: `make fmt lint test`, `go vet ./...`, `go build ./...`,
  `make build`, `make compatibility` e `make e2e` passaram em Linux. Coberturas:
  template **96,1%**, runner **94,9%**, CLI **98,0%**, suite **95,1%**,
  transport **90,4%**, report **100%**.
- `make compatibility`: **448 pares**, Hurl **8.0.0** obrigatório, sem skip,
  sem `-update`. São 440 históricos + 8 novos; somente 11 observações
  HTTPGo/classificações antigas foram alteradas deliberadamente. Nenhuma entrada
  `.hurl` histórica ou observação Hurl mudou. Novos goldens colhidos somente com
  `go test -tags compatibility ./tests/e2e -run '^TestCompatibility/renewal_' -count=1 -update -v`,
  revisados pelo wire e conferidos posteriormente no gate completo.
- Fuzzers executados **separadamente**, com seeds e oráculo de overlay/descarte:
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
  (**18.669** execuções) e o mesmo comando com `'^FuzzRenderJSON$'`
  (**69.932** execuções), ambos sem crash/falha.
- `git diff --check`, `gofmt -l` dos arquivos alterados e verificação dos links
  locais adicionados; ajuda e exemplo publicados cobertos pelo binário real.

### Rastreabilidade GO-459–GO-474

| GO | Evidência automatizada |
|---|---|
| 459 | E2E `TestCaptureRenewalDemonstration`: cenário versionado, seção normal/invertida, caminhos job-a/successor e job-b; PENDING encerra com 1 |
| 460–461 | template `TestStagedCaptureReplacement`, `TestDiscardedOverlaySharesOnlyProtection`; runner `TestRenewalCaptureSourcesAndRunIsolation`, `TestTransactionalCaptureAttempt`; negativos históricos mantidos para duplicação/hierarquia |
| 462–464 | runner `TestTransactionalCaptureAttempt`, `TestOwnCaptureLiteralExpected`; testes anteriores de status, templates JSON/header/body/regex/filtros adaptados à nova ordem, mantendo perfil literal |
| 465–466 | runner `TestTransactionalPolling`: lote novo apenas após aprovação; requests anteriores e métricas/callback terminal, sem confirmação parcial em divergência/avaliação |
| 467–468 | runner `TestDiscardedRenewalProtectsOldNewAndDerivedBeforeCallback`; CLI `TestInvocationProvisionalProtectionAndResolution`; E2E `TestCaptureRenewalSecretSuiteByBinary`, texto/XML e valores iniciais preservados no segundo arquivo |
| 469 | E2E `TestCaptureRenewalHelpAndLateValidation`, negativo diferencial `renewal_variable_unsupported`: sintaxe tardia/variable sem nenhum HTTP do arquivo |
| 470–472 | runner `TestCancelBeforeCaptureConfirmation`, `TestCaptureExtractionErrorsPrecedeAssertionsAndNeverRetry`; CLI `TestCaptureRenewalWriterFailures`; regressões de transporte/recursos existentes |
| 473 | reuso de runner/cenário, contexto inicial imutável, visão descartada e invocação nova sem estado de resolução/proteção anterior |
| 474 | 448 pares completos, incluindo 8 `renewal_*`: diferença observada em reenvios Hurl, encadeamento no lote, duplicação, erros e query ainda excluída |

### Limites e mudança pública

A ordem padrão e a substituição exata são mudanças **breaking** intencionais.
Hurl usa capturas reprovadas no próximo retry (body/header); HTTPGo preserva os
confirmados antes da entrada. Hurl encadeia capturas do lote e aceita duplicação
nos casos observados; HTTPGo não. Erro de captura não é condição de retry aqui.
Essas diferenças foram registradas, não eliminadas nem generalizadas para outras
versões. Códigos próprios 0/1/2, com 2 prioritário, permanecem.

Corpus sintético/HTTP local não homologa API/suíte real nem cenários do Borapedir.
macOS/Windows não executados (opcionais pelo plano v4). Não foram executados CI
remoto ou análise SonarQube. A proteção não detecta todo segredo arbitrário nem
codificações desconhecidas e não pode redigir retroativamente stderr já escrito.
A transação é de variáveis, não desfaz efeitos da API nem escrita de relatório.
Query `variable`, novas políticas de retry, encadeamento de capturas e mudanças
na entrada de credenciais permanecem excluídos.


## US-17 — issue #68 / história 0017

Plano da issue #68 executado sem planner, somente backend, nas três tasks
registradas e autoaprovadas na fila 0017. `back:1`: `7aa77a7` (renderer JUnit e
parser do consumidor); `back:2`: `b00dc13` (flags/destino/segurança e E2E);
`back:3`: esta alteração (workflow de referência, documentação e regressão).
Sem frontend/estáticos, dependências de runtime novas, alteração de Go, lint ou
contratos de parsing/execução. Nenhum arquivo em `testdata/compatibility/` mudou.
O novo workflow foi explicitamente previsto na task 3. Nenhum código-fonte Hurl
foi consultado/adaptado; backdev não executou SonarQube.

### Ambiente e comandos executados

Linux amd64, Go `go1.26.10-X:nodwarf5`; ferramentas existentes `goimports` e
`golangci-lint` selecionadas pelo PATH `/home/ff/go/bin`. Para o consumidor de
CI, Node 24 e npm, checkout isolado de `dorny/test-reporter` v2.6.0 no commit
`3d76b34a4535afbd0600d347b09a6ee5deb3ed7f`; `npm ci --ignore-scripts` e
`npm run build` executados sem alterar seu código-fonte. São dependências de
desenvolvimento/CI do consumidor, não do binário Go.

Passaram:

- Task 1: `go test ./internal/report`; golden gerado somente com `-update` e
  conferido sem atualização. `node tests/ci/junit-consumer.mjs <consumer>
  testdata/report/junit.xml` confirmou quatro testcases, um sucesso, dois
  falhos (`failure`/`error`), um skipped, Unicode/aspas/caracteres reservados,
  durações e diagnóstico exatamente preservados após parse.
- Task 2: `go test ./internal/cli ./internal/report` e
  `go test ./tests/e2e -run '^TestJUnit' -count=1`, com binário compilado e
  `httptest.Server`. Permissão negada testada em processo realmente não
  privilegiado (inclusive se harness roda como root); `/dev/full` comprova I/O
  real, sem limitar a validação a writer falso.
- Em **cada task com código**, `make fmt lint test`, `go vet ./...` e
  `make build` passaram antes do commit. Lint: **0 issues**; testes com
  **`-race -cover`**, report **100%**, CLI **97,9%** na validação integrada.
- Gate final da task 3: `go test ./tests/e2e -count=1`, `make e2e`,
  `make fmt lint test`, `go vet ./...`, `go build ./...`, `make build`,
  `git diff --check` e `gofmt -l` nos arquivos Go alterados passaram.
- Ingestão dos XMLs do binário:
  `go test ./tests/e2e -run '^TestJUnit' -count=1 -junit-artifact-dir=<dir>` e
  `node tests/ci/junit-execution.mjs <consumer> <dir>` passaram. Sucesso:
  **4/0/0**; misto: **2/2/0**; fail-fast de assertion: **1/1/2**;
  fail-fast de transporte: **2/1/1**, na ordem aprovados/falhos/skipped.
  O consumidor apresenta `failure` e `error` ambos como failed; o XML distingue.
- Regressão adicional `make compatibility`: **440 pares**, Hurl **8.0.0**
  existente selecionado pelo PATH, sem skip, atualização ou alteração de corpus.
- Fuzzing adicional de parser e template: `FuzzParse` e `FuzzRender` com
  `-fuzztime=5s -parallel=2`, **2.788** e **18.950** execuções, sem crash.
- YAML do workflow lido por parser YAML local; scripts de ingestão executados
  com o parser original que será usado pela action fixada.

### Rastreabilidade GO-170–GO-175

| GO | Evidência automatizada |
|---|---|
| 170 | `report.TestJUnitResults`, E2E `TestJUnitRealSuiteAndUnchangedRequests` e scripts `tests/ci/`: contagens, durações, identidade por arquivo e estados; modos contínuo e fail-fast |
| 171 | `cli.TestJUnitSuiteClassificationAndNoReexecution`, E2E acima e `TestJUnitDocumentedExample`: flags intercaladas/inline, criação de pais, artefato em falha, código preservado e mesma quantidade de requests que texto |
| 172 | `cli.TestJUnitPreexecutionErrorsLeaveDestinationUntouched`, E2E `TestJUnitRealPreexecutionAndDestinationErrors`: configuração/descoberta/cancelamento prévio sem HTTP nem relatório fictício; destino prévio intacto |
| 173 | `report.TestJUnitResults`, `TestJUnitUnknownStateAndInvalidXMLCharacters`, E2E de suíte/segredos e parser original: aspas/Unicode/`&`/`<` escapados, caracteres XML inválidos substituídos por U+FFFD |
| 174 | `report.TestJUnitWriteErrors`, `cli.TestJUnitDestinationFailures`, `TestJUnitDiagnosticFailureAndCancellationStillReport`, E2E `TestJUnitRealWriteAndPermissionFailures`: open/write/short/close, destino inválido/sem permissão, prioridade 2 e fechamento garantido |
| 175 | `cli.TestJUnitSecretExpectedAndActualOperands`, `TestJUnitProtectsEarlierDiagnosticsAndNames`, E2E `TestJUnitRealSecretsAndEscaping`: ambiente/capturas, operandos esperados/reais, headers sensíveis, nomes e descoberta posterior protegidos; valores enviados intactos |

### Limites da evidência

O [workflow](../.github/workflows/junit.yml) está configurado para ingerir,
reter e apresentar os artefatos reais, com verificação dos totais publicados.
**A publicação visual de checks no GitHub Actions não foi executada nesta sessão**:
não houve push nem disparo remoto. A evidência local é de ingestão efetiva pelo
parser original do mesmo consumidor fixado, não uma declaração de execução remota.
PRs de fork mantêm parser/artefatos, mas não publicação de check por token read-only.

Falhas sintéticas deliberadas no relatório de demonstração não são falhas dos
testes Go: os estados/exit codes/requests são verificados antes da publicação.
O workflow não relaxa os gates `make test/lint`. Corpus sintético não equivale à
homologação de API real; outros consumidores JUnit não foram certificados.
macOS/Windows não executados (opcionais nesta história pelo plano v4).
Segredos arbitrários não marcados/transformações desconhecidas permanecem fora
da garantia, assim como redação retroativa de stderr incremental. Escrita não é
transacional; uma falha de I/O pode deixar artefato parcial e sempre retorna 2.

## US-33 — issue #66 / história 0033

Plano da issue #66 executado sem planner, somente backend, nas três tasks
registradas e autoaprovadas da fila 0033. `back:1`: `d7db569` (captura numérica
ponta a ponta); `back:2`: `27d0ed3` (fronteiras/segurança/isolamento); `back:3`:
esta alteração (corpus, exemplos, ajuda/docs e validação integrada). Base
`dc63bbc`, com US-28/30/31 disponíveis. Sem frontend/estáticos, dependências
novas, alteração de Go, CI/lint ou transporte de produção. Nenhum código-fonte
Hurl foi consultado, copiado ou adaptado. Gates locais não substituem review;
backdev não executou análise SonarQube.

### Ambiente e evidências executadas

Linux amd64; Go `go1.26.10-X:nodwarf5`; `goimports`/`golangci-lint` existentes
em `/home/ff/go/bin`. Hurl **8.0.0** existente em
`/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, selecionado pelo
PATH em vez do padrão 8.0.1, sem instalar ferramentas. Aviso de metadados libcurl
não impediu identificação ou execução da versão obrigatória.

- Task 1: passaram `go test ./internal/query ./internal/assertion`,
  `go test ./internal/parser ./internal/template ./internal/runner ./internal/cli`,
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e
  `go build ./...` antes do commit.
- Task 2: passaram testes runner/CLI/suíte/transporte nos gates completos,
  E2E focalizado `go test ./tests/e2e -run '^TestCountCapture' -count=1`,
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e
  `go build ./...` antes do commit.
- Task 3 integrada: `make compatibility` passou com **440 pares**, Hurl
  **8.0.0** obrigatório, sem skip ou `-update` no gate completo. Os **16 novos**
  pares foram observados com atualização **restrita ao prefixo `capture_count_`**,
  revisados (inclusive wire tipado e classificações) e conferidos sem atualização.
  Nenhum dos **424 pares históricos** foi alterado; harness não foi modificado.
- `make fmt lint test` passou com **0 issues** e testes `-race -cover`;
  `go vet ./...`, `go build ./...`, `make build` e `make e2e` passaram.
  O teste antigo de ajuda que proibia capturas foi atualizado para o recorte
  US-33; a validação integral foi reexecutada com sucesso, sem afrouxar gates.
- Fuzzing passou com seeds ampliadas, sem crash:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
    — 29.135 execuções;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
    — 11.714 execuções;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
    — 20.508 execuções.

### Rastreabilidade GO-430–GO-444

| GO | Evidência automatizada |
|---|---|
| 430 | E2E `TestCountCaptureDemonstration`: preservada 0, alterada 1, conteúdo diferente com mesma quantidade 0 e próximo envio bloqueado; pares `unchanged`/`changed` |
| 431 | query `TestCountArray`, runner `TestCountCaptureSelectionAndTypedReuse`, E2E `TestCountCaptureNumericBodyWithoutRenderAsserts`: raiz/caminho simples, vazio 0, elementos null, wire numérico e textual |
| 432 | runner `TestCountCaptureComparisonsAndOrder`: cinco comparadores; pares `numeric`, `unchanged`, `empty` conferem tipo enviado e limites |
| 433 | runner `TestCountCaptureSelectionAndTypedReuse`: string/objeto/número/booleano/null/ausência são captura 2 sem publicação/avanço; CLI verifica consulta/count/causa/posição |
| 434–435 | mesmo runner: zero, escalar único, vários, null, array único inclusive vazio e vários arrays por wildcard/filtro, sem achatamento; corpus compara contagem transformada com Hurl |
| 436 | runner `TestCountCaptureComparisonsAndOrder`: status/primeira assertion prevalecem, captura própria indisponível e captura de entrada divergente não publicada |
| 437 | `TestCountCaptureAtomicity`, `TestCountCaptureConflictDirections`, `TestCountCaptureBoundaryFailures`: lote misto, inicial/anterior/mesmo lote, três direções de conflito, valores preservados, nenhum próximo envio |
| 438 | parser `TestCountCaptureSyntax`: LF/CRLF, consumo integral, consulta inválida e limites; E2E `TestCountCaptureDemonstration`/`TestCountCaptureDocumentation`: erro tardio e negativos sem HTTP |
| 439 | CLI `TestCountCaptureCodesAndSecretComparison`, runner `TestCountCaptureSecretComparisonCallback`/`TestCountCaptureBoundaryFailures`: esperado/real ocultos mesmo diferentes, proteção em lote rejeitado, pendente não avaliado e metadados seguros |
| 440 | E2E `TestCountCaptureNumericBodyWithoutRenderAsserts`: requisição tipada sem flag; `TestCountCaptureDemonstration` e par `no_optin`: esperado dinâmico rejeitado antes de HTTP |
| 441 | runner `TestCountCaptureSelectionAndTypedReuse`: vazio, JSON malformado/adicional, percurso incompatível singular ou ramo plural não viram zero/contagem parcial |
| 442 | transporte `TestCountCaptureFinalResponse`/`TestCountCaptureReadAndCloseFailure`, E2E `TestCountCaptureCompleteFinalResponse`: redirects finais, read/close, prazo/cancelamento e recursos fechados sem baseline parcial |
| 443 | runner `TestCountCaptureBoundaryFailures`, CLI `TestCountCaptureWriterPriority`: erro/short write sobre sucesso/divergência/captura/avaliação, sem reimpressão nem próximo envio; contagens preservadas |
| 444 | runner `TestCountCaptureRunnerIsolation`, suíte `TestCountCaptureSuiteIsolation`, E2E `TestCountCaptureSuiteNoSharedBaseline`: cenário/iniciais imutáveis, sessões/arquivos novos, resultados/callbacks sem coleção/planos de captura |

### Contrato e limites da evidência

A proibição antiga de `count` em captura JSONPath foi removida explicitamente;
`unsupported_capture_count.hurl` virou positivo `capture_count.hurl`, com
negativos específicos para pipeline/header. Não há aritmética, parâmetros,
contagem de strings/objetos ou novos comparadores. Classificação de `count` em
assertion continua 1 para ausência/tipo incompatível; em captura é 2.

Os pares de contagem por wildcard/filtro **com transformação** concordam com
Hurl nas quantidades observadas; isso não muda as diferenças anteriores de
captura **sem transformação**. Códigos 2/3 em captura e 1/4 em assertion são
próprios; colisão, captura na mesma resposta, pipelines e opt-in continuam
explicitamente diferentes. Matriz registra somente o observado, sem inferir
JSONPath completo ou equivalência geral de runners.

Exemplos publicados são comparados byte a byte com o cenário versionado em
`TestCountCaptureDocumentation`, que executa os dois positivos com binário/HTTP
reais e verifica ajuda/negativos. Links locais/âncoras novos foram conferidos.
Corpus sintético/local não equivale à homologação da suíte real. macOS/Windows
não executados, opcionais nesta história pelo plano v4; não são gates Linux nem
evidência de execução nativa nessas plataformas.

## US-32 — issue #64 / história 0032

Plano da issue #64 executado sem planner, somente backend, em três tasks
registradas/aprovadas na fila 0032. `back:1`: `5f137d8` (desigualdade literal e
dinâmica ponta a ponta); `back:2`: `f0fbb58` (segurança/falhas externas);
`back:3`: esta alteração (corpus, ajuda/docs, seeds e gates). Base `221f9ea`,
com US-08/30 disponíveis. Sem frontend/estáticos, dependências novas, alteração
de Go, CI/lint ou transporte de produção. Nenhum código-fonte Hurl consultado,
copiado ou adaptado. Gates locais não substituem revisão; não foi executado
SonarQube pelo backdev.

### Ambiente e comandos efetivamente executados

Linux amd64, Go `go1.26.10-X:nodwarf5`; `goimports` e `golangci-lint` existentes
em `/home/ff/go/bin`. Hurl **8.0.0** existente em
`/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, selecionado via
PATH (padrão local 8.0.1), sem instalação. Aviso de metadados libcurl não impediu
identificação/execução da referência obrigatória.

- Task 1: passaram `go test ./internal/assertion`,
  `go test ./internal/parser ./internal/runner ./internal/cli`,
  `go test ./tests/e2e -run 'TestJSON' -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/runner ./internal/cli ./internal/transport`,
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e
  `go build ./...` antes do commit.
- Entrega integrada: passaram `make compatibility` com **424 pares** e Hurl
  **8.0.0**, sem skip ou `-update`; `make fmt lint test` (**0 issues**, testes
  `-race -cover`), `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
  Links locais/âncoras Markdown de README/subset/architecture/compatibility
  conferidos e exemplos publicados comparados com o cenário executado em E2E.
- Fuzzing com seeds ampliadas, sem crash nos três comandos:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
    — 88.698 execuções;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
    — 12.678 execuções;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
    — 7.317 execuções.

### Rastreabilidade GO-415–GO-429

| GO | Evidência automatizada executada |
|---|---|
| 415 | `TestJSONInequalityRotation`: login/captura secreta/renovação/binário real, novo token 0, repetido 1, streams seguros e envio dependente bloqueado |
| 416 | assertion `TestJSONNotEqual`: matriz dos seis tipos/vazios; E2E `TestJSONInequalityValues` distingue número/string/null |
| 417 | assertion `TestJSONNotEqual` e E2E: 1/1.0/1e0 e zeros com sinal/expoente não diferem |
| 418 | assertion e E2E: chaves reordenadas iguais, mudança aninhada/ordem de arrays diferentes |
| 419 | runner `TestJSONInequalityOrderAndPublication`, E2E: propriedade/índice ausente falham 1, diagnóstico ausente distinto de null e sem publicação/avanço |
| 420 | mesmos testes: JSON inválido/percurso incompatível retornam 2, não sucesso por inversão |
| 421 | runner `TestJSONInequalityLiteralProfile`, `TestJSONInequalityOrderAndPublication`, CLI `TestJSONInequalityProfilesAndValidation`: literal com variável definida/ausente, tipos/escapes/precisão e não recursão |
| 422 | runner: variável ausente, captura própria/futura são erro ao alcançar; captura anterior utilizável no E2E de rotação e no teste de derivado |
| 423 | parser `TestJSONInequalityParsing`, CLI, E2E `TestJSONInequalityDocumentation`: LF/CRLF, HTTP obrigatório, esperado/query inválidos, negativos header/body/count/negação antes da rede |
| 424 | runner `TestJSONInequalityOrderAndPublication`: status/primeira assertion precedem esperados posteriores; esperado antes de decode/seleção; publicação atômica e erro posterior de captura |
| 425 | runner `TestJSONInequalitySafeOperands`, `TestJSONInequalityCapturedDerivative`, `TestJSONInequalityExternalFailures`: callbacks/resultados sem planos/operandos/coleções brutos; pendente oculta ambos sem avaliar captura; E2E valida streams |
| 426 | `TestJSONInequalityDocumentation`, corpus diferencial novo e gate integral: ajuda/exemplos, perfis e diferenças Hurl 8.0.0 |
| 427 | assertion/runner/E2E: inteiros além de float64, precisão aninhada e expoentes extremos, sem expandir potências |
| 428 | transporte `TestJSONInequalityRequiresReadAndClose`, `TestJSONInequalityFinalAndCancellation`, runner e E2E `TestJSONInequalityFinalResponse`: resposta final/read/close/prazo/cancelamento sem aprovar parcial ou avançar |
| 429 | runner `TestJSONInequalityExternalFailures`, CLI `TestJSONInequalityWriterPriority`, E2E Linux `TestJSONInequalityRealOutputFailure`: falha/short write mantém 2 sobre sucesso/divergência/avaliação, sem reimprimir ou próximo envio |

### Mudança de contrato e limites

**Mudança intencional em `testdata/compatibility/`:** somente a observação HTTPGo
(e classificação) de `assert_json_operator_unsupported.json` muda na base:
**2, zero envios → 0, /capture → /health**. Entrada e observação Hurl preservadas.
Adicionados **18 pares `inequality_*`**; outros **405 pares históricos** não
mudaram. Observações novas conferidas/corrigidas pontualmente a partir de execução
real e depois validadas integralmente sem regeneração indiscriminada.

Hurl 8.0.0 também falha por ausência; distingue os inteiros adjacentes usados
neste corpus; gramática rejeita objetos/arrays esperados diretos aceitos pelo
HTTPGo. Perfis literal/dinâmico divergem pela seleção explícita; percurso inválido
é erro 2 HTTPGo versus assertion 4 Hurl. Matriz publica essas diferenças, não
compatibilidade irrestrita. Segredos precisam de marcação explícita; `!=` não
comprova revogação/validade de tokens nem ausência de efeitos externos.

Corpus é sintético/local, não homologação de suíte real. Não houve builds/testes
macOS/Windows; opcionais nesta história conforme plano v4, sem bloquear o gate
Linux. Nenhuma evidência de execução nativa nessas plataformas é anunciada.

## US-31 — issue #62 / história 0031

Plano da issue #62 executado sem planner, somente backend, nas quatro tasks
registradas/aprovadas da fila 0031: `back:1` em `e8764bc` (seleção dinâmica tipada
ponta a ponta), `back:2` em `c18940c` (composição/cardinalidade/publicação),
`back:3` em `ec66269` (sigilo/isolamento/falhas externas) e `back:4` nesta alteração
(corpus, ajuda/docs, seeds e gates integrados). Dependências US-28/30 presentes
na base `0d65918`. Sem frontend/estáticos, novas dependências, alteração de Go,
CI/lint ou transporte de produção. Nenhum código-fonte Hurl consultado/copiado/
adaptado. Gates locais não substituem review; backdev não executou SonarQube.

### Ambiente e execução efetiva

Linux amd64 em **2026-11-17**, Go `go1.26.10-X:nodwarf5`, `goimports` e
`golangci-lint 2.15.0` existentes em `/home/ff/go/bin`. Hurl **8.0.0** existente
em `/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin` escolhido via
PATH (o padrão do ambiente é 8.0.1), sem instalar ferramentas. O aviso de
metadados de libcurl não impediu identificação/execução da referência 8.0.0.

- Task 1: passaram `go test ./internal/query ./internal/parser ./internal/template
  ./internal/runner ./internal/suite ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/query ./internal/assertion ./internal/template
  ./internal/runner`, `go test ./tests/e2e -count=1`, `make fmt lint test` e
  `go vet ./...` antes do commit.
- Task 3: passaram `go test ./internal/runner ./internal/suite ./internal/cli
  ./internal/transport`, `go test ./tests/e2e -count=1`, `make fmt lint test` e
  `go vet ./...` antes do commit, incluindo novo E2E de sigilo em streams.
- Task 4: passaram testes específicos de todos os pacotes alterados, E2E completo,
  `make compatibility` com **406 pares**, Hurl **8.0.0** obrigatório sem skip ou
  `-update` no gate completo; `make fmt lint test` (**0 issues**, `-race -cover`),
  `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
- Observações dos **19 pares novos `dynamic_filter_*`** foram colhidas com
  atualização restrita ao prefixo novo e depois conferidas sem `-update`.
  Só dois pares históricos mudaram, manual e explicitamente:
  `filter_literal_placeholder.json`/`filter_literal_defined.json`, parte HTTPGo
  (agora **2, zero envios**) e classificação. Entradas e observações Hurl desses
  pares não mudaram; os outros **385 pares da base** permaneceram inalterados.
- Seeds ampliadas nos dois perfis, LF/CRLF, duas fronteiras de escape e conteúdo
  adversarial. Passaram os comandos exigidos, com `-parallel=2` e sem crash:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
    — 177.733 execuções na última rodada registrada;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
    — 27.345 execuções;
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
    — 67.675 execuções.

### Evidência rastreável GO-396–GO-414

| GO | Testes/evidência executados |
|---|---|
| 396 | `TestDynamicFilterDemonstration`: binário/HTTP real, número anterior exato, pedido correto versus cancelado/ausente, captura/envio só após aprovação |
| 397 | `TestBindFilterOperands`, `TestDynamicFilterValuesAndOrder`, `TestDynamicFilterCaptureReuse`: string/número/bool/null, inteiros grandes, decimais próximos e expoente extremo sem float64 |
| 398 | `TestDynamicFilterValuesAndOrder`: aspas simples/duplas, texto vazio, números/bool/null e escapes escalares |
| 399 | query `TestBindFilterOperands`, runner `TestDynamicFilterValuesAndOrder`: arrays/objetos vazios completos/compostos são erro, mesmo em resposta vazia |
| 400 | query/runner tests e pares `injection_bare/quoted`, `escapes`, `nonrecursive`: valores não geram expressão nem nova resolução |
| 401 | `TestDynamicFilterValidation`, `TestDynamicFilterFileValidation`, `TestDynamicFilterStructuralCorpusByBinary`: consulta/propriedade/índice/caminho relativo/operador/concatenação excluídos, zero rede |
| 402 | `TestDynamicFilterInvocationIsolation`, parser/query opt-in, pares `no_optin` e históricos atualizados: literal com placeholder rejeitado sem flag, inclusive com braces codificadas por escape interno |
| 403 | parser `TestDynamicFilterFileValidation`: LF/CRLF e posições originais para ambas as declarações, sem resolver variável futura; E2E negativo tardio impede HTTP |
| 404 | runner `TestDynamicFilterValuesAndOrder`, `TestDynamicFilterOwnCaptureUnavailable`, `TestDynamicFilterCaptureAtomicity`, pares `missing/same_response`: ausência, própria/futura e arrays vazios |
| 405 | `TestDynamicFilterFirstFailure`: status/assertion anterior prevalece, esperado antes dos operandos, operandos antes de decode/seleção |
| 406 | `TestDynamicFilterCaptureAtomicity`: sem acesso ao lote provisório, publicação parcial ou sobrescrita; proteção descoberta permanece em resolução/seleção/colisão tardias |
| 407 | `TestDynamicFilterPredicatesAndFailures`, `TestDynamicFilterCaptureReuse`, E2E `TestDynamicFilterCaptureContracts`: redução, null, captura plural tipada e erro pré-envio textual |
| 408 | `TestDynamicFilterComposition`: filtros sobre propriedades/índices e wildcards aninhados, ordem, associação e arrays sem achatamento |
| 409 | `TestDynamicFilterSafeResultsAndReuse`, `TestDynamicFilterDerivedSecret`, `TestDynamicFilterPendingDerivedSecret`, `TestDynamicFilterSecretStreams`, suíte/CLI: proveniência, captura pendente, resultados/callbacks/streams sem plano/valor protegido |
| 410 | `TestDynamicFilterPublishedExample`, corpus de 19 pares, ajuda de raiz/test e `make compatibility`: evidência Hurl 8.0.0 e diferenças classificadas |
| 411 | `TestDynamicFilterPredicatesAndFailures`, E2E `TestDynamicFilterCaptureContracts`: JSON vazio/inválido/adicional e ramos incompatíveis não aprovam negação nem publicam seleção parcial |
| 412 | transporte `TestDynamicFilterFinalResponseAndCancellation` e `TestDynamicFilterReadCloseBeforeReport`, runner `TestDynamicFilterExternalFailureOrder`: final completa, read/close/prazo/cancelamento, recursos antes do callback |
| 413 | CLI `TestDynamicFilterSecretWriterPriority`: erro/short write após sucesso/divergência/resolução inválida mantém 2 e interrompe sem reimprimir no writer falho |
| 414 | `TestDynamicFilterSafeResultsAndReuse`, `TestDynamicFilterSuiteIsolation`, `TestDynamicFilterInvocationIsolation`: cenário imutável, valores por sessão e opt-in sem retenção na próxima invocação |

### Contratos e limites

**Breaking change explícita:** placeholder em literal de consulta não é mais
tratado como dado estático sem flag. A US-31 substitui essa permissão US-28;
o modo literal dos esperados US-30 permanece inalterado. Também foram fechadas
as fronteiras de consulta inteira dinâmica (erro não suportado) e placeholders
codificados em literais internos (opt-in necessário após decode). A inspeção de
referências validadas também protege captura pendente derivada de operando
secreto, sem resolver/avaliar sua consulta (mesmo contendo referência ausente).

O diferencial não anuncia equivalência integral: Hurl interpola sem flag,
admite índice dinâmico e diverge em precisão/escapes, cardinalidade de captura
e acesso à captura da mesma resposta. Vinculação tipada não reinterpreta valor
como expressão. O corpus é **sintético**, separado da homologação da suíte real.
`backend` indica área trabalhada, não aprovação automática da história.
macOS/Windows opcionais durante a história, conforme plano v4, sem alegar
execução nativa ou portabilidade validada nessas plataformas.


## US-29 — issue #58 / história 0029

Plano da issue #58 executado sem planner, somente backend, nas três tasks
aprovadas da fila 0029: `back:1` em `3315dc9` (predicados ponta a ponta),
`back:2` em `dd6d671` (seleção plural, sigilo e fronteiras), `back:3` nesta
alteração (compatibilidade, ajuda/documentação e validação integrada).
Sem frontend/estáticos, nova dependência, atualização de Go ou alteração de
CI/lint. Query, transporte e sessão de produção não foram modificados.
Nenhum código-fonte Hurl consultado/copiado/adaptado. Gates locais não substituem
review; backdev não executou SonarQube.

### Ambiente e execução efetiva

Linux amd64 em **2026-11-17**, Go `go1.26.10-X:nodwarf5`, ferramentas existentes
em `/home/ff/go/bin` (`goimports`, `golangci-lint`). Hurl **8.0.0** existente em
`/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, selecionado via PATH
sem instalar/substituir ferramentas. O aviso de metadados de libcurl não impediu
a identificação nem execução do binário de referência.

- Task 1: passaram `go test ./internal/parser ./internal/assertion ./internal/runner
  ./internal/cli`, `go test ./tests/e2e -run 'TestJSON' -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/query ./internal/assertion ./internal/runner
  ./internal/cli ./internal/transport ./internal/suite`, E2E JSON com binário/HTTP
  real e os mesmos gates `make fmt lint test`, vet e build antes do commit.
- Task 3: passaram testes específicos de parser/assertion/runner/CLI e E2E JSON;
  `make compatibility` com **367 pares**, Hurl **8.0.0** obrigatório, sem skip e
  sem `-update` no gate completo; `make fmt lint test` (**0 issues**, race/cobertura),
  `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
- Os **43 pares novos `types_*`** foram observados com atualização limitada a
  `go test -tags compatibility ./tests/e2e -run '^TestCompatibility/types_' -count=1 -update`,
  inspecionados e classificados individualmente antes do gate sem atualização.
  **Nenhum dos 324 pares anteriores mudou.** As novas construções são uma
  ampliação explícita do subconjunto, não mudança da igualdade/contagem existente.
- Observado 0/0 para a demonstração, false booleano, notação inteira negativa/-0,
  inteiros grandes e string/array/objeto vazios. Decimal/expoente/string numérica,
  whitespace, null, tipo errado e ausência divergem com HTTPGo 1/Hurl 4.
  Pluralidade preserva redução zero/um/vários, sem aplicação por item.
- Diferenças de percurso estrito e rejeição estática integral de `count isInteger`
  e negação fora do recorte estão classificadas em
  [compatibility.md](compatibility.md#tipos-e-vazio--us-29--issue-58).
  Aprovar o **tipo** de um inteiro grande não demonstra precisão de igualdade
  da referência. Não foi imposto limite de máquina para imitá-la.
- Cobertura integrada: assertion **96,5%**, CLI **97,5%**, config **96,0%**,
  parser **97,3%**, query **88,3%**, runner **94,6%**, suite **95,5%**,
  template **96,1%**, transport **90,4%**. Nenhuma meta/verificação reduzida.
- Seeds de parser ampliadas com unários válidos/inválidos, seleção simples,
  wildcard/filtro e LF/CRLF; seeds de JSON incluem grafias numéricas, false e
  vazios. Fuzzers executados e aprovados, sem crash:
  - `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
    (**121.910 execuções**);
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRender$' -fuzztime=20s -parallel=2`
    (**29.354 execuções**);
  - `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
    (**16.029 execuções**). Contagens correspondem apenas a esta rodada.
- `TestJSONTypeHelpAndPublishedExample` confere ajuda root/test e o exemplo
  versionado idêntico no README/subset, executando `/session-info` em HTTP local.
  `TestJSONTypeStructuralCorpusByBinary` cobre os novos arquivos negativos;
  testes estáticos LF/CRLF contam zero envios. Links locais, exemplos,
  `git diff --check` e `gofmt -l` conferidos.
- macOS/Windows não executados: opcionais nesta história conforme plano v4.
  Corpus sintético não homologa suíte real, schema ou JSONPath/Hurl integral.

### Rastreabilidade GO-361–GO-376

| GO | Evidência executada |
|---|---|
| 361 | `TestJSONTypeDemoAndCapture`, `TestJSONTypeByBinary`, `TestJSONTypeHelpAndPublishedExample`, par `types_demo`: quatro assertions aprovadas, false aceito e captura libera envio dependente |
| 362–363 | `TestJSONTypePredicates`, `TestJSONTypeByBinary`, pares `types_integer_*`, `types_string_*`, `types_boolean_*`: tipos sem coerção, -0/negativos/alta precisão, decimal/expoente reprovam |
| 364–365 | Mesmos tabelados, pares `types_empty_*`: vazios na raiz passam; whitespace, null em coleção, null/número/bool não passam |
| 366 | `TestJSONTypeOrderAndFailures`, `TestJSONTypePredicates`, pares `types_*_missing`: ausência/índice ausente falha sem captura/avanço |
| 367 | `TestJSONTypeAfterReduction`, `TestJSONTypeReducedSelection`, E2E e pares `types_plural_*`: zero/um/vários e arrays sem flatten, nunca predicado por elemento |
| 368 | `TestJSONTypeDeclarations`, `TestJSONTypeInvalidNeverSends`, corpus negativo e pares `types_late_*`: categorias e posições LF/CRLF, validação integral antes da rede |
| 369 | `TestJSONTypeOrderAndFailures`: status e ordem escrita prevalecem, header anterior evita decode inválido; `HTTP *` dispensa somente status nos E2E |
| 370 | `TestJSONTypeProtectedResultsAndReuse`, `TestJSONTypeProtectedMetadataCLI`, `TestJSONTypeSuiteProtectionAndIsolation`: valores/estruturas e metadados derivados ocultos, captura pendente, planos removidos, callbacks/cenário reutilizados sem mutação |
| 371 | `TestJSONTypePredicates`, `TestJSONTypeByBinary`, `types_equality_notation` e regressões existentes: igualdade exata, quantidade integral, limites e matches inalterados |
| 372 | `TestCompatibility` (367 pares, 43 novos), ajuda root/test e cenário publicado executados, diferenças classificadas sem generalizar compatibilidade |
| 373–374 | `TestJSONTypeOrderAndFailures`, `TestJSONTypeReducedSelection`, `TestJSONTypeByBinary`, pares `types_strict_*`: JSON vazio/malformado/adicional e ramo incompatível são erro 2, sem parcial/ausência falsa |
| 375 | `TestJSONTypeFinalAndInterruptedResponse`, `TestJSONTypeReadCloseBeforeReport`, `TestJSONTypeFinalAndIncompleteByBinary`: resposta final completa, read/close/prazo/cancelamento e recursos encerrados antes do report, sem captura/avanço |
| 376 | `TestJSONTypeCallbackFailure`, `TestJSONTypeWriterFailureOverridesOutcomes`: callback/erro/short write em aprovação/divergência/avaliação mantém 2, contagens/resultados anteriores, sem reimpressão/envio seguinte |

## US-30 — issue #60 / história 0030

Plano da issue #60 executado sem planner, somente backend, nas quatro tasks
aprovadas da fila 0030: `back:1` em `fe79c50` (opt-in e comparação JSON tipada),
`back:2` em `82089f3` (texto, pertencimento e regex), `back:3` em `797f9f6`
(isolamento, sigilo e falhas externas), `back:4` nesta alteração (perfis/corpus,
ajuda/documentação e gates integrados). Sem frontend/estáticos, atualização de
Go/dependências, CI/lint ou transporte de produção. Nenhum código-fonte Hurl
consultado/copiado/adaptado. Gates locais não substituem review; backdev não
executou SonarQube. `backend` descreve área, não aceite final da história.
Os hashes e as contagens abaixo registram a execução original, anterior ao rebase.

### Revalidação após rebase com develop

Rebase sobre `origin/develop` em `cfcb0c8` (US-29 integrada). Conflitos de
README/arquitetura/compatibilidade/validação e seeds de parser/JSON resolvidos
preservando ambas as histórias, sem alterar nenhum par de contrato existente.
O corpus integrado passa a **387 pares**: 367 da base e 20 da US-30. O perfil
opt-in não modifica os predicados unários sem esperado da US-29.

Passaram novamente testes específicos de parser/template/assertion/runner/suite/
CLI/transport, E2E JSON e render-asserts; `make compatibility` com Hurl **8.0.0**
obrigatório e sem `-update`; `make fmt lint test` (**0 issues**, race/cobertura),
`go vet ./...`, `go build ./...` e `make e2e`. Coberturas após integração:
assertion **96,7%**, CLI **97,2%**, parser **97,4%**, runner **95,2%**,
suite **95,1%**; demais pacotes mantêm as porcentagens originais abaixo.
Os três fuzzers foram repetidos com `-fuzztime=20s -parallel=2`, todos aprovados:
`FuzzParse` **142.369**, `FuzzRender` **30.133**, `FuzzRenderJSON` **8.250**
execuções. As seeds das duas histórias foram preservadas e o parser continua
exercitando ambos os perfis. Nenhuma análise SonarQube foi executada pelo backdev.

### Ambiente e execução original (antes do rebase)

Linux amd64, **2026-11-17**, Go `go1.26.10-X:nodwarf5`, goimports e
golangci-lint **2.15.0** existentes em `/home/ff/go/bin`. Hurl **8.0.0** existente
em `/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, selecionado via
PATH sem instalar ferramentas nem substituir a versão padrão 8.0.1. O aviso de
metadados libcurl não impediu identificação/execução da referência.

- Task 1: passaram `go test ./internal/parser ./internal/template
  ./internal/assertion ./internal/runner ./internal/suite ./internal/cli`,
  `go test ./tests/e2e -run TestJSON -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram testes específicos parser/template/assertion/runner/CLI,
  `go test ./tests/e2e -count=1`, `make fmt lint test`, `go vet ./...` e
  `go build ./...` antes do commit.
- Task 3: passaram `go test ./internal/runner ./internal/suite ./internal/cli
  ./internal/transport`, `go test ./tests/e2e -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit.
- Task 4: passaram parser/template/CLI e E2E de ajuda/exemplos/perfis,
  `make compatibility` com **344 pares**, Hurl **8.0.0** obrigatório, sem skip
  ou atualização no gate completo; `make fmt lint test` (**0 issues**, race e
  cobertura), `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
- **20 pares novos `render_asserts_*`** foram observados e revistos com
  atualização restrita a `TestCompatibility/render_asserts_`, seguida de gate
  completo sem `-update`. **Nenhum dos 324 pares anteriores foi alterado**.
  `httpgo_profile` ausente/literal preserva o comando antigo; `render-asserts`
  passa a flag só ao HTTPGo. As árvores/sessões e fixtures são independentes.
  Precisão, tipos, padrões, primeira ocorrência, literal padrão e publicação
  estão classificados em [compatibility.md](compatibility.md#perfis-de-esperados--us-30--issue-60).
- Evidência real: ambos distinguem o par numérico 9007199254740992/9007199254740993
  no esperado dinâmico, sem inferir precisão geral Hurl; string esperada difere
  do número. JSON composto dinâmico é aceito aqui, mas Hurl rejeita sua gramática.
  Captura da própria resposta é erro 2 aqui e disponível no Hurl. Nenhuma
  divergência foi ocultada ou normalizada para declarar equivalência completa.
- Cobertura integrada: assertion **95,8%**, CLI **97,2%**, config **96,0%**,
  parser **97,4%**, query **88,3%**, runner **94,7%**, suite **95,1%**,
  template **96,1%**, transport **90,4%**. Sem redução de verificações/limites.
- Fuzzing com seeds novos e parser exercitando **ambos os perfis**:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**111.793 execuções**), `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'
  -fuzztime=20s -parallel=2` (**38.039 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**66.513 execuções**). Todos passaram sem crash; contagens desta rodada.
- Ajuda root/test, exemplo versionado/README/subset e negativos são conferidos
  por `TestRenderAssertsHelpAndPublishedExample` e
  `TestRenderAssertsStructuralCorpusByBinary`. Teste legado de ajuda de body foi
  atualizado para exigir o perfil literal e a flag, não mais negar o opt-in.
  Links locais, exemplos, `git diff --check` e `gofmt -l` revisados.
- macOS/Windows não executados: opcionais nesta história pelo plano v4.
  Corpus sintético não homologa suíte real nem anuncia Hurl/JSONPath integral.

### Rastreabilidade GO-377–GO-395

| GO | Evidência executada |
|---|---|
| 377 | `TestJSONRenderAssertsDemonstration`, exemplo publicado e par `capture`: POST/captura/GET com ID exato, 0/1/2 e bloqueio de captura/envio após falha |
| 378 | `TestRenderAssertsJSONValues`, `TestRenderAssertsJSONParsing`, `ValidateJSONExpected`: tipo completo, precisão, estruturas aninhadas, string escapada e ausência de coerção |
| 379 | `TestRenderAssertsLiteralProfileByBinary`, `TestRenderAssertsInvocationIsolation`, pares `literal_*` novos e corpus antigo intacto: placeholders definidos/ausentes permanecem literais, bare rejeitado sem flag |
| 380 | `TestRenderAssertsJSONValues`, `TestValidateJSONExpected`, pares count/ordered: zero, 1.0/1e0, quantidade além de inteiro de máquina e comparação exata |
| 381 | tabelas JSON/texto/parser: esperado inválido é avaliação 2; tipo real incompatível segue divergência 1; arrays/objetos não viram texto |
| 382 | `TestRenderAssertsTextEvaluation`, `TestRenderAssertsTextByBinary`, pares text/membership/header_repeat: substring, pertencimento tipado, negação e primeira ocorrência |
| 383 | tabelas textuais/E2E e pares regex: expressão interpolada sem QuoteMeta, regex final inválida/fora da engine é avaliação 2 |
| 384 | casos não recursivos JSON/texto/regex e par nonrecursive: dado inserido não exige variável mencionada por seu conteúdo |
| 385 | `TestRenderAssertsJSONOrder`, suite isolation, pares missing/same_response/capture: somente iniciais/capturas anteriores publicadas |
| 386 | `TestRenderAssertsJSONOrder`, `TestRenderAssertsQueryErrorPrecedence`: status/assertion anterior prevalece; esperado alcançado precede consulta/decode/negação |
| 387 | tabelas parser LF/CRLF e `TestRenderAssertsStructuralCorpusByBinary`: template/escape/conteúdo adicional/regex estática inválidos impedem todo HTTP |
| 388 | tabelas parser, negativos versionados e par unsupported_body_regex: flag não amplia operadores, consultas, nomes nem regex de captura |
| 389 | `TestRenderAssertsInvocationIsolation`, `TestRenderAssertsSuiteIsolation`, `TestRenderAssertsSuiteIsolationByBinary`: comando reutilizado, todos os arquivos, sessões/capturas isoladas, fail-fast/continuação e prioridade 2 |
| 390 | `TestRenderAssertsJSONSecret`, `TestRenderAssertsTextSecrets`, pending secrets/scenario reuse, callbacks suite e writer CLI: segredo/derivados ocultos e planos/regex removidos; cenário original não mutado |
| 391 | `TestCompatibility` com 344 pares, 20 novos perfis explícitos; ajuda root/test, exemplo publicado e diferenças observadas classificadas |
| 392 | `TestRenderAssertsQueryErrorPrecedence`: JSON inválido/percurso incompatível/charset não viram ausência ou aprovação de negação |
| 393 | tabelas texto/regex: string vazia, null/booleano/número textuais, ausência distinta e estruturas rejeitadas |
| 394 | `TestRenderAssertsCompleteFinalResponse`, `TestRenderAssertsReadAndCloseFailure`, `TestRenderAssertsTransportAndReportFailures`, E2E final/incomplete: final de redirect, read/close/prazo/cancelamento sem preparação parcial/captura/avanço |
| 395 | `TestRenderAssertsSecretsAndWriterPriority`: erro e short write em sucesso/divergência/erro de esperado prevalecem como 2, sem reescrever writer nem próximo envio |

## US-28 — issue #56 / história 0028

Plano da issue #56 executado sem planner, somente backend, nas quatro tasks
aprovadas da fila 0028: `back:1` em `eef1637` (filtros estáticos com igualdade
exata ponta a ponta), `back:2` em `5703923` (composição/cardinalidade/reuso tipado),
`back:3` em `1fc7b28` (proteção e falhas externas), `back:4` nesta alteração
(corpus diferencial, ajuda/documentação e gates integrados). Sem frontend,
estáticos, alteração de Go/dependências/CI/lint ou transporte de produção.
Nenhuma fonte Hurl consultada/copiada/adaptada; somente comportamento do binário
8.0.0. Gates locais não substituem review; backdev não executou SonarQube.

### Ambiente e execução efetiva

Linux amd64 em **2026-11-17**, Go `go1.26.10-X:nodwarf5`, golangci-lint **2.15.0**
e goimports existentes em `/home/ff/go/bin`. Hurl **8.0.0** já existente em
`/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, escolhido via PATH,
sem instalar ferramentas nem substituir a versão padrão 8.0.1. O aviso de
metadados de libcurl não impediu identificação/execução dos pares.

- Task 1: passaram `go test ./internal/query ./internal/parser ./internal/assertion
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `make build` antes do commit.
- Task 2: passaram `go test ./internal/query ./internal/assertion ./internal/template
  ./internal/runner ./internal/cli`, `go test ./tests/e2e -count=1`,
  `make fmt lint test` e `go vet ./...` antes do commit.
- Task 3: passaram `go test ./internal/runner ./internal/cli ./internal/transport
  ./internal/suite`, `go test ./tests/e2e -count=1`, `make fmt lint test` e
  `go vet ./...` antes do commit.
- Task 4: passaram os testes específicos query/parser/assertion/runner/CLI e
  `go test ./tests/e2e -run '^TestFilter' -count=1`; `make compatibility` com
  **324 pares**, Hurl **8.0.0** obrigatório, sem skip ou `-update` no gate completo;
  `make fmt lint test` (**0 issues**, race/cobertura), `go vet ./...`,
  `go build ./...`, `make build` e `make e2e`.
- **31 pares novos `filter_*`** foram observados/revistos com fixtures independentes
  e atualização limitada a `TestCompatibility/filter_`, seguida de verificação
  sem atualização. **Nenhum dos 293 pares anteriores foi alterado**. São mudanças
  explícitas de linguagem os filtros de igualdade; negativos históricos de
  existência permanecem. O cenário negativo de projeção com igualdade foi
  substituído intencionalmente por `!=`, sem alterar seus pares de compatibility.
- Evidência diferencial mostrou diferenças reais de precisão (inteiros acima de
  2^53 e decimais próximos), percurso estrito, placeholders e reuso composto.
  Expoente extremo causou abort do binário Hurl por overflow de expoente, observado
  como exit -1/sinal em Linux; HTTPGo preserva igualdade exata. Classificação e
  dados enviados permanecem em [compatibility.md](compatibility.md#filtros-estáticos-por-igualdade--us-28--issue-56).
  Não se simulou equivalência nem se mudou o avaliador conforme o resultado.
- Na integração final, ampliada a classificação de recursos relativos excluídos
  e operandos dinâmicos como não suportados, sem confundi-los com literal
  malformado. A inspeção de literais **decodificados** também passou a ocultar
  resumos derivados de filtro protegido escapado, mesmo com valores selecionados
  públicos; regressão inspeciona estruturas/callbacks além dos streams.
- Cobertura integrada: assertion **95,3%**, CLI **97,5%**, config **96,0%**,
  parser **97,2%**, query **88,3%**, runner **94,1%**, suite **95,5%**,
  template **96,1%**, transport **90,4%**. Não houve meta/limite reduzido.
- Fuzzing com seeds adicionais de filtros/escapes e comandos executados:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$' -fuzztime=20s -parallel=2`
  (**104.932 execuções**), `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'
  -fuzztime=20s -parallel=2` (**35.987 execuções**) e
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$' -fuzztime=20s -parallel=2`
  (**10.465 execuções**). Todos passaram sem crash; contagens são desta rodada.
- Ajuda root/test, exemplo versionado e bloco de escapes documentados são
  executados por `TestFilterHelpAndPublishedExample`, `TestFilterByBinary` e
  `TestFilterPublishedEscapes`. Negativos são executados sem rede por
  `TestFilterStructuralCorpusByBinary`. Links locais novos e coerência dos
  exemplos foram conferidos; `git diff --check` e `gofmt -l` sem pendências.
- macOS/Windows não executados: opcionais nesta história segundo plano v4.
  Corpus sintético não homologa suíte real nem JSONPath/RFC integral.

### Rastreabilidade GO-344–GO-360

| GO | Evidência executada |
|---|---|
| 344 | `TestFilterByBinary`, `TestFilterEntityAndSafeFailures`, pares entity/reordered/inactive: ID correto independentemente da ordem e outro item ativo não aprova pizza inativa |
| 345–346 | `TestFilterTypedEquality`, `TestFilterEqualityAndAssertionAgree`, `TestFilterDeclarations`, `TestFilterPublishedEscapes`, pares typed/precision/escape: igualdade tipada, ausência/null, precisão além de int64, expoentes e escapes em dois níveis |
| 347–348 | `TestFilterCompositionAndCardinality`, `TestFilterNestedCaptureByBinary`, pares nested/cardinality: mesmo item, navegação/filtro alternados, ordem/repetições, zero/um/vários e arrays sem flatten |
| 349–350 | `TestFilterCaptureTypedReuse`, `TestFilterPredicatesAndOrder`, pares null/missing/plural capture/text: null válido, ausência 2, JSON completo exato e coleção nunca vira texto/envio dependente |
| 351 | `TestFilterPredicatesAndOrder`, pares scalar_count/numeric_equivalent: count/igualdade/presença sobre valor reduzido, sem predicado implícito por elemento |
| 352 | `TestFilterValidation`, `TestFilterDeclarations`, `TestFilterInvalidLateNeverSends`, `TestFilterStructuralCorpusByBinary`: categorias distintas, posições originais LF/CRLF e zero envios do arquivo |
| 353–354 | `TestFilterPredicatesAndOrder`, `TestFilterCaptureAtomicAndProtected`: status e primeira assertion prevalecem; lote não publica parcialmente nem sobrescreve, proteção descoberta persiste |
| 355 | `TestFilterPlansAndLiteralsNeverReachReports`, `TestFilterLiteralPresentation`, `TestFilterProtectedDiagnosticsCLI`, `TestFilterFileIsolationAndSafeCallbacks`: plano/literais/documento removidos, resumos protegidos, callbacks/cenário reutilizados e sessões isoladas |
| 356 | `TestCompatibility` (324 pares, 31 novos), `TestFilterHelpAndPublishedExample`, `TestFilterPublishedEscapes`: recorte e diferenças reais documentados; regressões GO-445–458 da US-27b preservadas |
| 357–358 | `TestFilterStrictEvaluation`, `TestFilterEntityAndSafeFailures`, `TestFilterPredicatesAndOrder`: incompatibilidade mista e JSON vazio/malformado/adicional são erros 2, nunca ausência/parcial/negação aprovada |
| 359 | `TestFilterFinalResponseAndCancellation`, `TestFilterReadCloseBeforeReport`, `TestFilterFinalAndIncompleteByBinary`: final de redirect, leitura/close incompleto, prazo/cancelamento e encerramento de recursos sem captura/avanço |
| 360 | `TestFilterCallbackFailureKeepsCounts`, `TestFilterWriterFailureOverridesOutcomes`: erro/short write 2 em sucesso/divergência/erro, contagens preservadas sem reescrita nem envio seguinte |

## US-27b — issue #54 / história 0027b

Plano da issue #54 executado sem planner, somente backend, na ordem das três
tasks aprovadas: `back:1` em `367be14` (adapter híbrido e precisão), `back:2`
em `acd4f7f` (atomicidade, sigilo e fronteiras), `back:3` nesta alteração
(documentação, corpus negativo e gates integrados). Acrescentada somente a
dependência autorizada `github.com/ohler55/ojg v1.28.9`, encapsulada em query.
Sem alteração de Go, frontend/estáticos, transporte de produção, CI/lint ou
contrato público. Nenhum código-fonte Hurl consultado/copiado/adaptado; observação
apenas do binário de referência. Gates locais não substituem review nem QA;
backdev não executou SonarQube.

### Ambiente e execução efetiva

Linux amd64, em **2026-11-17**, Go `go1.26.10-X:nodwarf5`, golangci-lint **2.15.0**
e goimports existentes em `/home/ff/go/bin`. Hurl **8.0.0** existente em
`/tmp/httpgo-0027-hurl/hurl-8.0.0-x86_64-unknown-linux-gnu/bin`, selecionado via
PATH para o gate, sem substituir a versão padrão 8.0.1 nem instalar ferramentas.
O aviso de metadados de libcurl não impediu execução/identificação da referência.

- Task 1: passaram `go mod tidy`, `go test ./internal/query ./internal/parser
  ./internal/assertion`, `go test ./internal/runner ./internal/cli`,
  `go test ./tests/e2e -run '^TestProjection' -count=1`, `make fmt lint test`,
  `go vet ./...` e `go build ./...` antes do commit.
- Task 2: passaram `go test ./internal/template ./internal/runner ./internal/cli
  ./internal/suite ./internal/transport`, `go test ./tests/e2e -count=1`,
  `make fmt lint test`, `go vet ./...` e `go build ./...` antes do commit.
- Task 3: passaram `make compatibility` (**293 pares**, Hurl **8.0.0** obrigatório,
  sem skip ou `-update`), `make fmt lint test` (**0 issues**, race/cobertura),
  `go vet ./...`, `go build ./...`, `make build` e `make e2e`.
  **Nenhum arquivo de `testdata/compatibility/` foi alterado ou regenerado**;
  cardinalidade, captura composta, precisão e diferenças estritas anteriores
  continuam com as mesmas observações. Dois novos negativos versionados cobrem
  função e interpolação estrutural tardias, sem rede.
- Cobertura integrada: assertion **94,2%**, CLI **97,5%**, config **96,0%**,
  parser **97,2%**, query **96,7%**, runner **94,1%**, suite **95,5%**,
  template **96,1%**, transport **90,4%**. E2E usa binário real e HTTP local,
  verificando dados enviados, stdout/stderr e exit code.
- Fuzzing preservado, comandos executados com `-fuzztime=20s -parallel=2`:
  `go test ./internal/parser -run '^$' -fuzz '^FuzzParse$'` (**108.576 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRender$'` (**21.813 execuções**),
  `go test ./internal/template -run '^$' -fuzz '^FuzzRenderJSON$'` (**6.016 execuções**).
  Todos passaram sem crash; contagens são específicas desta rodada.
- Ajuda root/test, exemplos versionados e negativos executados pelos testes
  `TestProjectionHelpAndPublishedExample`, `TestProjectionPrecisionDemo` e
  `TestProjectionStructuralCorpusByBinary`. Links locais novos, diff e
  `gofmt -l` dos arquivos Go alterados conferidos.
- macOS/Windows não executados: opcionais nesta história pelo plano v4, não
  bloqueios. Corpus sintético não homologa suíte real, filtros ou RFC integral.

### Rastreabilidade GO-445–GO-458

| GO | Evidência executada (novos testes e regressões existentes) |
|---|---|
| 445 | `TestProjectionPrecisionDemo`: IDs distintos 0 com POST exato; segundo ID repetido 1, sem captura/POST |
| 446 | `TestProjectionCompiledSelectionIsStatic`, testes simples de query/parser: caminhos nativos, raiz/nomes/índices e distinção legada Select/SelectValue |
| 447 | `TestProjectionExactNumbers`, `TestProjectionExactNumberCapture`, `TestProjectionCaptureReuse`, `TestTypedCaptures`, comparações exatas de assertion: além de int64, decimais próximos, formas equivalentes e expoentes extremos, tipos/grafias mantidos |
| 448–449 | `TestProjectionCardinality`, `TestProjectionPredicatesAndOrder`, `TestProjectionReducedTypesByBinary`, pares existentes: redução única, ordem/repetições, null/ausência, arrays sem flatten, count/presença/pertencimento sobre valor reduzido |
| 450 | `TestProjectionStrictTraversal`, `TestProjectionPredicatesAndOrder`, `TestProjectionExactBatchAtomicAndProtected`, pares strict: erro inteiro sem parcial ou negação aprovada |
| 451 | `TestProjectionCaptureReuse`, `TestProjectionExactBatchAtomicAndProtected`, `TestProjectionCollectionTextNeverSends`, `TestTypedCaptures`: null válido, ausência erro, coleção tipada sem uso textual, lote atômico e proteção descoberta |
| 452 | `TestProjectionDeclarations`, `TestProjectionInvalidLateNeverSends`, `TestProjectionStructuralCorpusByBinary`: assertions/capturas tardias LF/CRLF, posições originais, sintaxe inválida versus não suportada e zero envios |
| 453 | `TestCompatibility` com 293 pares inalterados e Hurl 8.0.0; ajuda/exemplos e documentação do limite de adoção |
| 454 | `TestProjectionInvalidJSONPrecedence`, `TestProjectionPredicatesAndOrder`: JSON vazio/malformado/adicional só quando alcançado, status/header anterior prevalece |
| 455 | `TestProjectionPlanRemovedFromEveryReport`, `TestProjectionExactBatchAtomicAndProtected`, `TestProjectionSafeFailure`, testes CLI/suite: todo Path zerado, operandos/coleções/números secretos ocultos, proteção de captura pendente/descoberta |
| 456 | `TestProjectionFinalResponseAndCancellation`, `TestProjectionReadCloseBeforeReport`, `TestProjectionFinalAndIncompleteByBinary`: redirect final, read/close/timeout/cancel, recursos e nenhuma captura/envio de parcial |
| 457 | `TestProjectionCallbackFailurePreservesCounts`, `TestProjectionWriterFailureOverridesOutcomes`: erro/short write prioritário 2 em sucesso/divergência/erro, sem reescrita ou próximo envio |
| 458 | `TestProjectionFileIsolationAndSafeCallbacks`, `TestProjectionPlanRemovedFromEveryReport`, `TestProjectionReportsAndSessionsIndependent`, `TestProjectionPublishedValuesDoNotCrossRuns`, `TestProjectionSuiteProtectionAndIsolation`: planos/cenários/documentos imutáveis, sem retenção operacional ou captura entre sessões, proteção acumulada só na invocação |

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
