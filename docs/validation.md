# Validação — US-01 a US-05

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
