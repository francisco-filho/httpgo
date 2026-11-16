# Arquitetura implementada — US-01 a US-16, US-20 a US-26

Referência de aceite: issues GitHub #4 (US-01), #6 (US-02), #11 (US-03), #13 (US-04), #14 (US-05), #16 (US-06), #18 (US-07), #20 (US-08), #22 (US-09), #24 (US-20) e #25 (US-10), #27 (US-11) e #29 (US-12) e #32 (US-13) e #34 (US-14) e #36 (US-15) e #38 (US-16),
#40 (US-21), #42 (US-22), #44 (US-23), #46 (US-24), #48 (US-25),
#50 (US-26) e plano v4 referenciado nelas.
Implementação independente baseada em requisitos/documentação pública, sem
copiar, adaptar ou traduzir código-fonte Hurl.

## Polling por entrada — US-26 / issue #50

`internal/parser/options.go` define `EntryOptions`, armazenado em `Entry.Options`:
`Retry` inteiro limitado para acomodar a tentativa inicial, `RetryInterval`
ponteiro de duração (nil distingue default de zero explícito) e `Position`.
`Interval()` aplica 1000 ms somente na ausência. `parseOption` aceita apenas
inteiros estáticos não negativos dentro dos limites de int/duração, sem opções
repetidas. Estados em `parser.go` associam `[Options]` à requisição, antes da
resposta, sem confundir opções com headers, JSON, multipart ou entrada seguinte.
Erro estático tardio descarta todo o cenário antes da rede.

`Runner.RunWithOptions`, em `internal/runner/polling.go`, cria uma sessão por
arquivo, acumula proteção com `protectScenario` e chama `pollEntry` por entrada.
`runAttempt`, em `runner.go`, prepara a requisição completa pelos preparadores
existentes, chama `HTTPExecutor.Execute`, fecha fontes, protege headers sensíveis,
compara status, avalia assertions e só então publica capturas atomicamente.
Retorna fatos privados da tentativa, sem callback ou alteração permanente do
resultado lógico. A divergência transitória não contamina a tentativa seguinte.
`pollEntry` só repete `Diverged` sem `Err` e com orçamento restante. Falha de
captura/preparação/avaliação/transporte/cancelamento encerra com categoria 2.

A espera privada `Runner.wait` é por instância e recebe contexto/duração;
`waitInterval` usa timer com `Stop` garantido, seleção cancelável e checagem do
contexto após o timer. Nova checagem antecede o envio, inclusive com intervalo
zero ou conclusão simultânea do timer/cancelamento. Não há `time.Sleep`, relógio
global, goroutine produtora ou novo deadline total. O executor concreto não mudou:
cada chamada cria seu prazo HTTP, cobrindo redirects e consumo completo.

Arquivos externos são relidos e multipart reaberto em cada tentativa, preservando
caminhos, variáveis, URL, headers e campos. Não há snapshot, buffer integral de
upload nem promessa de bytes iguais após alteração externa. A fonte multipart é
fechada antes da avaliação/espera; cada tentativa pode gerar novo boundary.
Redirects pertencem à tentativa atual; a seguinte começa na URL/método originais. POST opt-in pode repetir
efeitos: sem idempotência prometida ou replay implícito no transporte.

`runner.Summary` conserva sua unidade lógica. `runner.AttemptSummary` fica
separado em `Result.Attempts`, `suite.FileResult.Attempts` e
`suite.Summary.Attempts`, com `Started` (chamadas ao executor), `Completed`
(respostas completas) e `StatusChecks` (comparações numéricas). `EntryResult.Attempts`
mostra a quantidade na única entrada/callback terminal. Fatos lógicos são os da
última tentativa enviada: preparação falha não inventa envio nem apaga o último
fato HTTP. Cancelamento no intervalo mantém métricas e produz `Cancelled`, não
uma divergência final 1. A suíte agrega tentativas separadamente e só classifica
o desfecho terminal; recuperação não aciona fail-fast. CLI rotula os novos campos
sem formato de relatório novo; falha de escrita continua prioritária (2).

Respostas descartadas não ficam em coleção/histórico. Segredos conhecidos e
variantes permanecem protegidos na mesma sessão/writers da invocação; capturas
secretas pendentes não são antecipadas. `safeEntry` sanitiza a nova posição e
copia o ponteiro do intervalo, sem expor fontes/bodies/coleções operacionais.
O cenário permanece imutável; orçamento e capturas não vazam entre execuções.

## Fluxo e arquivos reais

```text
cmd/httpgo/main.go
  → internal/cli/command.go + arguments.go (flags/contexto novo e proteção)
    → internal/config/config.go + value.go + compose.go + environment.go + reference.go (YAML, composição e resolução explícita)
    → internal/suite/discovery.go (descoberta completa, identidade e ordenação)
    → internal/suite/run.go (carga por arquivo, continuação, agregação compacta)
    → internal/parser/parser.go + capture.go + assertion.go + multipart.go + options.go (arquivo inteiro)
      → internal/query/jsonpath.go (validar consultas antes da rede)
    → internal/runner/runner.go + polling.go + render.go (somente cenário validado)
      → internal/template/template.go + json.go (valores tipados, texto/JSON contextual)
      → parser.ValidateURL/ValidateHeader e validação JSON final
      → internal/runner/file.go (leitura integral e fechamento de body externo)
      → internal/runner/render.go (JSON externo opt-in com a mesma sessão)
      → internal/runner/multipart.go + multipart_mime.go (fonte aberta e textos preparados)
      → internal/transport/http.go + multipart.go (via HTTPExecutor)
      → fechamento da fonte multipart antes da avaliação/apresentação
      → internal/assertion/status.go (somente expectativa numérica)
      → internal/runner/assertions.go (headers/JSON/body na ordem escrita, antes de capturas)
        → internal/query/header.go + internal/assertion/header.go
        → internal/query/jsonpath.go + internal/assertion/json.go (sob demanda)
        → internal/query/body.go + internal/assertion/body.go (texto sob demanda)
      → internal/query/jsonpath.go (selecionar JSON da resposta final)
      → internal/runner/capture.go (serializar seleção JSON sem float64)
      → internal/template/session.go (proteger e publicar JSON tipado atomicamente)
      → apresentação segura de cada resultado na CLI (callback síncrono)
    → resumo por arquivo na CLI antes do próximo arquivo
  → resumo agregado da suíte na CLI
```

O diagrama é fluxo de execução; `runner` **não importa** o adapter HTTP.
Não há framework de CLI, pacote genérico de tipos, domínio rico ou novos pacotes
reservados para capacidades futuras. A US-11 acrescenta somente `internal/config`
e a dependência YAML mantida `go.yaml.in/yaml/v3` **v3.0.6**, isolada nesse pacote.
A US-21 acrescenta `golang.org/x/text v0.44.0`, encapsulada em `query` para
rótulos/conversões de charset. O restante usa biblioteca padrão; `go.mod` preserva módulo
`capimgrosso.com/httpgo` e Go `1.26.10`.

| Pacote/tipos | Responsabilidade |
|---|---|
| `parser.Position`, `Header`, `Assertion`, `HeaderAssertion`, `JSONAssertion`, `Capture`, `Entry`, `Scenario`, `Error` | Modelo ordenado com método, URL, headers, body, capturas e posições de declaração/consulta; estados de requisição, expectativa e capturas; validação completa sem cenário parcial |
| `runner.Request`, `Response`, `HTTPExecutor` | Contrato mínimo definido no consumidor, sem net/http: `Execute(context.Context, Request) (Response, error)`; Request carrega Method, URL, Headers (`[]parser.Header`), Body (`[]byte`), Multipart (`*MultipartBody`), RetainBody e Timeout (`time.Duration`); Response retorna Status, Headers (`[]query.HeaderValue`, ocorrências ordenadas) e Body final retido somente quando solicitado |
| `runner.Runner`, `EntryResult`, `Outcome`, `Summary`, `Result` | Ordem, contexto antes de cada chamada, comparação delegada e interrupção; resultado estruturado e notificação síncrona por entrada, sem escrever em streams nem chamar os.Exit |
| `template.Context`, `Text`, `Reference`, `Error` | Contexto tipado isolado, nomes exatos/conflitos, offsets, substituição não recursiva e proveniência; não importa parser/runner/CLI/transporte |
| `template.JSONPlan`, `JSONReference` | Reconhecimento de strings/escapes/valores e chaves estáticas, referências com intervalos originais, limite exato do body; compartilhado por parser e renderer |
| `template.SecretVariants`, `Protector` | Proteção de valores brutos, decodificados e escapes conhecidos, inclusive antes de validar contexto; sem substituição de texto vazio |
| `runner.Renderer`, `RenderError`, `CaptureError` | Factory consumer-owned `NewSession() template.Session` por Run; erros pré-envio e de captura com categorias/origens distintas; clones preservam cenário |
| `template.Session`, `Capture`, `ConflictError`, `CaptureValueError` | Renderização e publicação atômica de JSON validado (`Capture.Value` é `json.RawMessage`); erro de payload/conflito indexa o lote sem expor conteúdo; variáveis iniciais imutáveis, cópia dos valores, capturas separadas e proteção dos valores descobertos |
| `query.Path`, `query.Error`, `query.Decode` | Pacote folha sem interface: validar JSONPath limitado; decodificar exatamente um JSON com UseNumber; SelectValue distingue presença/ausência/percurso incompatível e Select mantém contrato de capturas |
| `query.HeaderValue`, `query.Header` | Ocorrências nome/valor sem net/http; seleção pura da primeira, com presença distinta de vazio e nome case-insensitive |
| `assertion.Status`, `HeaderOperator`, `assertion.Header`, `assertion.JSONEqual`, `assertion.MatchString` | Comparação de status/headers, igualdade JSON tipada e correspondência regex de string sem coerção; não conhece runner, HTTP ou apresentação |
| `parser.JSONAssertion.Pattern`, `Compiled` | `matches` conserva literal de padrão e `*regexp.Regexp` validado durante parse integral; padrão não passa por renderer e é removido dos metadados pelo `safeEntry` |
| `parser.HeaderAssertion.Expected`, `Compiled` | US-24: padrão decodificado em `Expected` (sem fonte duplicada) e `*regexp.Regexp` validado pelo helper privado `parseMatches`, compartilhado com JSON; `safeEntry` clona a declaração e remove `Compiled` antes de apresentar |
| `runner.HeaderFailure`, `JSONFailure`, `JSONError` | Divergências e erros de avaliação localizados; nome/consulta, esperado/recebido sanitizados e presença explícita; sem coleção/body/documento bruto |
| `transport.NetHTTPExecutor`, `Error`, `ErrorKind` | HTTP real, prazo, redirects, consumo/fechamento e classificação segura com causa preservada via Unwrap |
| `suite.Suite`, `File`, `FileResult`, `State`, `Summary`, `Result`, `Options` | Descoberta completa, carga/fechamento por arquivo, reutilização do runner com sessão nova, estados e contagens compactos, fail-fast e interrupções globais; não importa CLI ou transporte |
| `cli.Command`, `BuildInfo` | Argumentos/streams (`SetArgs/SetIn/SetOut/SetErr`), configuração única, montagem da suíte, saída segura e exit code; não reavalia assertion |
| `main.run` | Monta dependências, `signal.NotifyContext`, streams do processo; seus defers terminam antes do único os.Exit de produção em main |

`AGENTS.md` registra a exceção autorizada para `transport` importar **os tipos**
`runner.Request/Response`, implementando o contrato consumer-owned sem camada
adicional nem ciclo. Testes de integração podem montar `runner.New` com o adapter.
Somente o código de produção em `transport` importa `net/http`.

## Inclusão e ausência — US-23

`parser/assertion.go` amplia os operadores existentes com `JSONContains`,
`JSONNotContains`, `JSONNotExists` e `HeaderNotContains`. Combinações de `not`
são fechadas, sem flag de negação genérica, AST, camada ou dependência nova.
Valida escalar JSON esperado, string em header e ausência de operando em
`not exists` no arquivo inteiro. Rejeita combinações com count e consultas
ampliadas; esperado e consulta não passam pelo renderer.

`query` permanece intacto: `Path.SelectValue` distingue seleção, presença e erro
de percurso; `Header` consulta a primeira ocorrência sem distinguir caixa.
`assertion/structure.go` separa ausência, aplicabilidade e condição. Ausência
aprova as duas negações JSON; tipo presente incompatível falha em pertencimento
nos dois sentidos. Strings usam `strings.Contains`; arrays comparam cada elemento
direto com `JSONEqual`, preservando números exatos, sem JSON textual, busca
recursiva, flatten ou índice. `not exists` olha só presença, inclusive para null.
`assertion/header.go` permite ausência somente para `HeaderNotContains`.

`JSONEvaluation.Actual` é nil para os novos predicados: não leva coleção/string
selecionada à serialização de falha. `runner/assertions.go` produz `presente`
para `not exists` e somente `tipo <tipo JSON>` para pertencimento, com esperado
escalar sanitizado. Reutiliza `JSONFailure`, `HeaderFailure`, `JSONError`, outcomes
e CLI sem formato novo ou reavaliação. `pendingSecret` oculta operandos antes de
callback/coleção; `safeEntry` remove esperado bruto e plano de consulta sem mutar
o cenário. Documento/arrays são locais à avaliação, nunca cache entre sessões.

Decode JSON continua uma vez por resposta, sob demanda. Status → assertions
na ordem escrita → capturas → callback síncrono permanece inalterado. As novas
assertions JSON já acionam RetainBody; header isolado continua descartando body,
mas somente após consumo/fechamento completo. Erro de decode/percurso é `2`,
condição falsa é `1` e erro de apresentação tem prioridade `2`. Transporte,
contexto/prazo único, redirects finais e ausência de retry não foram alterados.
Sem cookie jar, `header not exists`, negação de igualdade/matches/count, wildcard,
filtros ou antecipação das US-27/28. Corpus e diferenças observadas estão em
[compatibility.md](compatibility.md#inclusão-e-ausência--us-23--issue-44).

## Texto do body — US-21

`parser.Assertion.Body` aponta para `BodyAssertion` (Operator, Expected,
Position), validada no arquivo inteiro por `parser/assertion.go`, na mesma lista
ordenada das variantes Header/JSON. Reutiliza strings literais sem renderer.
`assertion.Body` compara substring com `strings.Contains` e sua negação; não
conhece charset/HTTP ou apresentação. Vazio é texto existente, não ausência.

`query.BodyText` seleciona o primeiro Content-Type, interpreta MIME e resolve o
charset no índice web de `x/text/encoding/htmlindex`; omitido usa UTF-8 estrito.
Valida UTF-8 e pares surrogate UTF-16; para outros decoders que podem substituir
sequências ilegais, a presença de U+FFFD exige validar somente os bytes que
produziram esse caractere, sem round-trip dos demais caracteres. Isso evita
rejeitar grafias alternativas válidas de GB18030 ao lado de um U+FFFD literal.
U+FFFD originalmente válido não é falha. Causas são categorias controladas, sem
rótulos, bytes ou mensagens brutas da biblioteca; sem sniffing ou fallback.

`runner/render.go` retém body também com assertion exclusivamente textual.
Transporte permanece intacto: consome e fecha resposta final inteira no orçamento
existente antes de retornar. Headers isolados continuam sem retenção.
`checkAssertions` retorna `error` para distinguir `JSONError` de `BodyError`.
Decodifica texto na primeira assertion de body e o reutiliza só naquela entrada,
independentemente do JSON; uma falha anterior prevalece sobre charset inválido.
Não cria cache global, concorrência ou nova interface de executor.

`BodyFailure` contém posição, operador e esperado sanitizado; `BodyError`, posição,
operador e causa categórica. Nunca carregam texto/body/fragmentos recebidos.
`pendingSecret` oculta o esperado antes de callback/coleção, sem avaliar capturas.
`safeEntry` clona a variante e remove Expected, preservando o cenário original.
CLI só apresenta e mapeia falha para 1/erro para 2; `ReportErr` mantém prioridade 2
e a suíte conta `BodyMismatch` como falha de assertion, não erro de transporte.
Sem pacote `report`, nova fonte de segredos, atualização de Go ou mudança de CI.

## Configuração do projeto — US-11

`config.Load(context.Context, path)` seleciona o explícito ou somente
`.httpgo.yaml` do cwd. `config.Settings` guarda `Variables map[string]json.RawMessage`,
`Secrets []string` e `Timeout time.Duration`. `load` recebe uma função privada de
abertura para testes de falhas, sem filesystem público ou estado global.
Lê com `io.LimitReader(MaxBytes+1)`, fecha inclusive em falha e descarta conteúdo
parcial; `Error` conserva causas I/O para `errors.Is/As` e só apresenta texto
controlado, origem com quoting e posição (1:1 quando indisponível).
Ausência opcional/vazio mantém defaults, falhas explícitas não têm fallback.

O decoder YAML produz **nós**, nunca unmarshal irrestrito em objetos. Antes de
converter, valida árvore completa, chaves únicas/string, aliases/âncoras/tags,
documento único e recursos YAML proibidos. A validação contextual de referências
da US-13 é feita nas folhas de variables, separadamente da conversão JSON. O schema aceita somente version 1,
defaults.timeout, variables e secrets. Mensagens brutas da biblioteca, trechos,
valores e nomes arbitrários não são expostos. `flatten` converte mapas variables
em caminhos usando `template.ValidName/NamesConflict`; arrays são valores JSON
completos, incluindo objetos internos. Conversão numérica trabalha na grafia,
sem float64, inclusive plain numbers que o resolver YAML classificaria como string
por exceder faixa de máquina. Strings entre aspas mantêm seu tipo.
`timeoutValue` calcula milissegundos exatos por dígitos/expoente decimal sem
expandir potências arbitrárias, verifica faixa 1..86400000 e só então cria Duration.

`Settings.Compose` valida CLI separadamente com `template.New`, aplica overrides
por nome exato e valida conflitos finais com o mesmo renderer. Não cria uma nova
linguagem de template, navegação de objetos ou merge de listas. Marcações são a
união YAML/CLI; `Settings.Protection` protege variantes dos valores conhecidos
da base e de todos os substitutos, inclusive em erros anteriores à sessão.
A CLI conecta essa proteção aos writers **antes** de reportar erros de argumentos,
contexto, composição ou cenário. `template.Context.NewSession` copia as proteções
iniciais e conserva o mecanismo dinâmico já existente para capturas/derivados.
Nada vaza para o comando/executor ou execução seguinte; capturas não sobrescrevem
valores iniciais. `Scenario.OriginDir` continua sendo o diretório do `.hurl`.

`runner.Options.Timeout` é metadado por execução, copiado para `Request.Timeout`
antes de Execute. Zero preserva o default existente de dez segundos. CLI fornece
o timeout efetivo e o usa no diagnóstico. O adapter escolhe o limite da request,
cria um contexto filho único antes do loop e mantém esse orçamento para conexão,
TLS, redirects e consumo/fechamento. Dial/TLS não possuem mais teto fixo de dez
segundos; o contexto menor/cancelamento do chamador prevalece. O executor não é
mutado. Não há goroutines novas nem promessa de interromper I/O local bloqueado.

Configuração inválida termina com 2 antes de parsing/HTTP. Não há overrides
`HTTPGO_*`, stdin ou hooks. `${NOME}` segue o fluxo US-13 abaixo. YAML é extensão
HTTPGo, com testes próprios separados do diferencial Hurl e no máximo 1 MiB lido.

## Ambientes nomeados — US-12

`cli.arguments.environment` reconhece `--env`/`--env=nome`, registra ocorrência
para rejeitar repetição mesmo idêntica e usa `config.ValidEnvironmentName`.
`config.LoadEnvironment(ctx, basePath, name)` reutiliza `loadDocument` e o decoder
de nós da US-11, com somente a versão opcional diferenciada por fonte. Ausência
de seleção não carrega ambientes. Nome ASCII não vazio é validado antes do
`filepath.Join(filepath.Dir(basePath), "environments", name+".yaml")`; base vazia
conserva sua raiz e ausência da base opcional usa cwd. Não muda cwd nem origem do
cenário. O ponto privado `loadEnvironment(..., open)` permite falhas de leitura/
fechamento, arquivos ausentes e cancelamento sem interface pública de filesystem.
Recursos são fechados antes da conversão/HTTP, com leitura limitada a MaxBytes+1.

`Settings.paths` conserva `variablePath` para mapas explícitos/implícitos e folhas,
inclusive mapas vazios, arquivo e nó com posição da fonte. `declarePath` rejeita
colisões antes de perder a hierarquia na conversão em nomes exatos. O renderer
continua sem conhecer YAML ou ambientes. `mergeEnvironment` compara estruturas
antes de aplicar os valores: mapa/valor incompatível é erro na origem do ambiente.
Mapas combinam por caminho; folhas são substituídas integralmente, incluindo
listas e null, sem reinterpretar strings. `Settings.Compose` ainda valida a CLI
separadamente, consulta a estrutura e só então aplica overrides por nome exato.
Objetos CLI permanecem opacos; erro anterior não pode ser escondido por override.

`Settings.sources` retém os valores conhecidos de ambas as fontes antes de
substituir folhas e `Secrets` une marcações. A proteção dos writers é montada
antes de apresentar erro de composição/argumento/cenário, incluindo valores
anteriores, substitutos CLI e variantes. Lista vazia não desprotege. Capturas e
proteção dinâmica seguem na sessão já existente, sem compartilhar estado entre
invocações. `timeoutSet` diferencia omissão de timeout do valor default: ambiente
só substitui quando declarado. O prazo efetivo segue pelo fluxo US-11, sem mudar
parser, transporte ou seus contratos. Nenhuma dependência, goroutine ou camada
adicionada. Referências explícitas de processo seguem a US-13 abaixo.

Testes `TestEnvironment*` em config/CLI/E2E cobrem GO-141–GO-160; extensões
continuam separadas do corpus Hurl. Evidências por cenário em `validation.md`.

## Referências de processo — US-13

`config.Settings.references` associa nomes de destinos achatados a
`processReference` (nome do processo, arquivo, linha e coluna), separadamente dos
valores JSON. `flatten` aceita a referência completa somente em folha string,
valida nome ASCII e acrescenta o destino a `Secrets` antes de qualquer override.
`jsonValue` rejeita referências aninhadas em listas; `rejectReferences` recusa-as
fora de variables. Nenhum valor inválido é ocultado por fonte posterior. As demais
proteções do decoder YAML e o limite de leitura permanecem iguais.

`mergeEnvironment` substitui referências por nome exato, acumulando marcas e
fontes de proteção antes de detectar conflito estrutural. `Settings.Resolve(ctx,
assignments)` cria cópia dos valores, descarta referências substituídas pela CLI,
ordena os destinos e consulta somente as efetivas. O ponto privado por instância
`Settings.lookupEnv` permite fake com contagem; produção usa `os.LookupEnv`, nunca
Getenv/Environ. Distingue vazio de ausência e serializa sempre com json.Marshal
string antes de template.New, sem interpretar JSON, `${OUTRA}` ou `{{outra}}` do
conteúdo. Não há nova dependência, interface pública de ambiente ou estado global.

A resolução coleta os valores efetivos disponíveis mesmo se outra referência
estiver ausente. Retorna Settings para proteção, mas `resolveErr` impede que
Compose publique um contexto parcial. Erros são `config.Error` com nome de
referência, chave e posição da fonte, nunca valores ou conteúdo bruto.
`Settings.Compose` também resolve para consumidores diretos; na CLI, os metadados
já foram consumidos por Resolve, sem nova consulta. Settings original é imutável;
uma próxima invocação carrega/responde ao ambiente atual.

`cli.Command.executeContext` carrega base/ambiente, resolve e conecta
`Settings.Protection` antes de apresentar erro de argumento, carga, resolução,
composição ou parsing. `protectedValues` inclui fontes originais, valores
resolvidos e substitutos marcados; reutiliza SecretVariants/Protector e
Context.ProtectValue/NewSession. Valores reais ficam somente no caminho operacional.
Runner já sanitiza operandos de headers/JSON e metadados antes de callback/coleção,
sem reavaliar assertions ou criar report/sanitizador novo. As sessões conservam
proteção dinâmica de capturas/derivados e isolamento, timeout e prioridade de
falha de escrita. Contexto é checado durante preparação e antes do envio; não há
goroutine, enumeração de ambiente, dotenv, stdin ou promessa de interromper I/O
local bloqueado. Testes `TestProcess*` cobrem GO-161–GO-178; YAML continua extensão
HTTPGo fora do diferencial Hurl.

## Arquivos externos — US-09

`parser.FileBody` conserva `Path` literal e `Position`, ligado por
`Entry.ExternalBody` não nulo (inclusive para arquivo vazio). `Entry.Body` não
nulo continua representando JSON inline; ambos nulos **e Entry.Multipart nulo**
significam ausência de body. O parser reconhece `file,` antes de headers e da rejeição genérica, inclusive
caminhos com `:`. Valida estrutura inteira/exclusividade/GET, sem I/O do payload.

`cli.Command` calcula a origem absoluta antes de abrir o `.hurl`, associa
`Scenario.OriginDir` e preserva `Scenario.File` nos diagnósticos. Na entrada
corrente, runner renderiza URL/headers e resolve o arquivo com `filepath.IsAbs`
e `filepath.Join`. `readPayload` abre, verifica diretório, lê integralmente e
fecha antes do executor, inclusive em leitura parcial/falha. `readAndClosePayload`
compõe erros com `errors.Join`; `FileError.Unwrap` conserva as causas para
`errors.Is/As`, mas seu texto expõe somente categorias controladas. `Runner.readFile`
é um ponto privado de substituição por instância para testes determinísticos;
não há interface pública de filesystem nem dependência nova.

No modo bytes padrão, o conteúdo externo não passa por `BodyPlan`,
`Session.RenderJSON` ou validação JSON: é entregue como `Request.Body []byte`. Transport não conhece caminhos e
continua responsável somente por HTTP/redirects/consumo/fechamento de resposta.
Template não conhece arquivos. Não há sandbox, tamanho máximo ou inferência de
Content-Type; memória adicional acompanha o payload corrente, sem retenção nos
resultados. A leitura usa permissões locais e pode alcançar fora do projeto.

Contexto é verificado antes/depois da leitura e antes do executor, sem goroutines.
Uma leitura local bloqueada pelo SO não é interrompida pelo timeout de transporte.
Erro local gera `FileFailed`, `Result.Err *FileError` e posição da referência;
callback síncrono recebe metadados seguros, e `safeEntry` remove `ExternalBody`,
body e headers. Não há bytes carregados nem caminho operacional no resultado.
A falha pré-envio não aumenta Started/Completed e preserva resultados anteriores.
Headers sensíveis já renderizados entram na proteção antes da leitura, inclusive
para sanitizar DisplayURL e writers se o arquivo falhar antes do transporte.
CLI reconhece FileError sem classificar como transporte/renderização nem imprimir
erro bruto/linha do cenário. ReportErr ainda prevalece com código 2.

## JSON externo opt-in — US-10

`cli.arguments.renderFiles` reconhece a flag sem valor `--render-files` após
`test`, antes/depois do arquivo, e a encaminha a `Runner.RunWithOptions` como
`runner.Options{RenderFiles: true}` por valor. `Run` conserva o modo bytes padrão
via opções zero. Nenhum estado mutável do runner/comando retém a ativação; cada
chamada continua criando sessão nova. HTTPExecutor/Request.Body e o adapter de
transporte não mudam; não há novos pacotes, interfaces de filesystem ou dependências.

Após URL/headers, somente para `Entry.ExternalBody` e com a opção ativa, o runner
resolve/lê/fecha como na US-09 e chama `renderExternal` em `render.go`. Essa função
reutiliza `Session.RenderJSON` com variáveis iniciais/capturas anteriores, exige um
valor completo e valida defensivamente o JSON final antes do executor. `template`
continua sem conhecer arquivos; o parser conserva referência literal, sem abrir
ou pré-validar payload. JSON estático preserva bytes externos, inclusive whitespace.
Não há seleção por extensão/Content-Type, inferência de header, interpolação de
caminhos/chaves ou renderização de arquivos multipart. O fluxo inline permanece
inalterado; multipart continua no caminho incremental da US-20.

Conteúdo inválido, inclusive vazio/template malformado, produz `RenderFailed` e
`RenderError`, separado de `FileFailed`/`FileError` para I/O. `RenderError.Position`
parte de 1:1 do payload e mapeia offsets sobre os bytes originais LF/CRLF; sem
offset confiável (inclusive defesa do JSON final), permanece 1:1. O campo opcional
`RenderError.Reference` identifica a declaração no `.hurl`. Nomes são sanitizados
pela sessão antes de entrar em resultado/callback; a CLI aplica quoting e proteção
nos writers. Nunca se usa o caminho absoluto operacional calculado ou posição do
resultado expandido. Causas brutas e payload não entram no texto de diagnóstico.

Contexto é verificado antes/depois da leitura, depois da renderização e antes do
executor. Não há nova goroutine, retry ou interrupção prometida para leitura local
bloqueada pelo SO. Recursos fecham antes do envio mesmo no modo JSON; memória
acompanha somente o payload corrente e sua renderização. Erros são tardios e locais:
contagens pré-envio, resultados anteriores, callback síncrono e prioridade de
ReportErr (2) permanecem. `safeEntry` remove bodies, planos e referências;
não se gera body de diagnóstico ou detecção arbitrária de segredos no arquivo.
A proteção existente da sessão acompanha capturas secretas/derivados e variantes
escapadas nos callbacks e writers, sem novas fontes de credenciais.

## Multipart incremental — US-20

`parser.MultipartForm` conserva `Position` e `Parts []MultipartPart` em ordem.
Cada parte tem nome, texto ou `File *FileBody`, MIME e posições de nome/valor.
`Entry.Multipart` distingue formulário de ausência de body, inclusive vazio do
arquivo. Parser reconhece a seção antes das rejeições genéricas e separa campos
de headers/JSON/capturas/assertions. Sem abrir arquivos, valida métodos, exclusividade,
nomes únicos, exatamente um arquivo, templates textuais e metadados estáticos.
Content-Type estático proibido rejeita todo o cenário; dinâmico espera a entrada.

`Runner.prepareMultipart` renderiza todos os textos via `Session.Render` e registra
proteção dos derivados antes de callback/diagnóstico. Arrays/objetos não viram
texto. Valida Content-Type final e basename e usa `multipartMIME` determinístico
(sem base MIME do SO ou sniffing); abre e verifica a fonte com origem da US-09.
`Runner.openUpload` é um ponto **privado por instância** para testes de I/O;
a interface privada `uploadFile` usa `io.ReadSeekCloser` e `Stat`, sem filesystem
público nem novo pacote. `runner.MultipartBody` contém partes operacionais,
`Source io.ReadSeeker` e tamanho; não contém caminhos e nunca entra em resultado.

Runner é dono do descritor. Mantém-no aberto durante `HTTPExecutor.Execute`, fecha
antes de status/assertions/capturas/callback e antes da próxima entrada, inclusive
em erro. Falha pré-envio não conta `Started`; falha de fechamento não confirma
`Completed` e preserva `FileError` localizado e causa via `errors.Is/As`. Falha de
writer ainda prevalece com 2. `safeEntry` remove **todo** `Entry.Multipart` além
dos bodies/headers anteriores. Não são retidos textos, arquivo ou metadados locais.

Transporte gera um boundary por requisição lógica, preserva os outros parâmetros
válidos de Content-Type (proibindo qualquer boundary manual) e compõe framing com
`mime/multipart.Writer` e `io.MultiReader`, mantendo em memória campos/metadados e
framing, não o arquivo. `uploadReader` serializa Read/Close e sinaliza fechamento:
o adapter aguarda o leitor HTTP terminar antes de retornar ou rebobinar, pois
`RoundTrip` pode retornar antes do fechamento assíncrono do request body. Não há
goroutine produtora, filas, concorrência entre entradas ou retry. Transporte é
dono desses leitores, mas não fecha nem abre o descritor local. Falha de leitura
da fonte é preservada; encerramento normal de envio após resposta completa, como
413 antecipado, não vira erro artificial de upload.

307/308 rebobinam somente depois desse encerramento e reutilizam o boundary;
301/302/303 descartam formulário/Content-Type e seguem como GET sem body. O loop
existente conserva dez redirects de mesma origem, TLS verificado e o mesmo contexto
com timeout efetivo para todos os hops e consumo da resposta. Cancelamento encerra a
atividade de rede; I/O local bloqueado pelo SO tem a limitação já documentada na
US-09. Bodies JSON inline e externos completos continuam em `[]byte` como antes.

## Apresentação incremental

A CLI monta `runner.NewWithRenderer(executor, sessionRenderer, c.renderEntry)`.
O adapter `sessionRenderer` delega a factory do contexto e conecta somente a
sanitização dinâmica aos writers locais da invocação.
`runner.New` conserva o construtor estático e usa contexto vazio. Após classificar cada resposta
ou falha, o runner chama o callback **sincronamente, antes de iniciar a próxima
entrada**. `renderEntry` escreve uma linha sanitizada, sem reavaliar assertions.
Sucessos, inclusive sem expectativa, começam com o método declarado; arquivo/linha/coluna
prefixam somente divergências e falhas. O resumo continua identificando o arquivo.
Não há goroutine, fila de eventos ou espera pelo fim do arquivo para apresentar
um resultado. Callback `nil` permite coletar resultados sem apresentação.

Ao terminar ou interromper o arquivo, `renderFile` escreve o resumo uma única
vez e `renderDiagnostic` apresenta sua causa sem decidir continuação. Classificação
fica em `suite`, separada da apresentação: `renderSuite` escreve o agregado e a
CLI retorna `suite.Result.Code`. Resultados já emitidos não são reimpressos.
O resultado estruturado e suas contagens continuam disponíveis no retorno do runner.

Se o callback falhar, `Result.ReportErr` preserva a causa e o runner não inicia o
envio seguinte. A CLI diagnostica a falha de escrita em stderr com texto controlado,
retorna 2 (também se houve divergência) e não tenta escrever o resumo no stdout
que falhou. `Result.Err` carrega transporte/cancelamento, `*runner.RenderError`
ou `*runner.CaptureError`/`*runner.JSONError`/`*runner.FileError`;
a CLI distingue a categoria com `errors.As`, sem ecoar causas brutas.

## Validação em duas etapas e segredos

`template.Parse` identifica referências e offsets em bytes. O parser associa-os
às posições iniciais de URL (`Entry.URLPosition`) e valores de headers
(`Entry.HeaderPositions`), conserva os textos originais e valida a estrutura
completa antes da rede. A request-line permite whitespace dentro dos placeholders,
não espaços arbitrários ou comentários inline. URLs/headers estáticos continuam
validados pelo parser. `template.ParseJSON` reconhece o intervalo do body e
valida a estrutura com placeholders substituídos por sentinelas válidas conforme
seu contexto. `Entry.BodyPosition` e `BodyPlan` conservam origem/referências;
`Entry.Body` conserva bytes originais. Fechamentos adjacentes de objetos não passam
pelo scanner textual; chaves dinâmicas são rejeitadas. Estruturas inválidas nunca
retornam cenário parcial, inclusive em entradas posteriores.

A CLI valida atribuições e conflitos antes de abrir/executar o cenário.
Cada chamada `ExecuteContext` cria contexto e streams protegidos locais, sem
acumular valores/marcações entre execuções. `argumentProtection` coleta declarações
independentemente da ordem/validade das flags, evitando eco de valores protegidos
até em erros anteriores à criação do contexto. Seu writer detecta short writes
como falha de escrita, preservando interrupção e prioridade do código 2.

`template.New` interpreta literais JSON válidos ou mantém fallback textual.
Objetos/arrays ficam opacos; strings são decodificadas e números mantêm sua
representação JSON, sem float64, inclusive nas estruturas. Nomes pontuados
resolvem por correspondência exata, nunca navegação ou merge de objetos.

Antes do executor, `template.Session.Render` resolve URL/headers sem recursão e
rejeita estruturas em contexto textual. `parser.ValidateURL`/`ValidateHeader`
reutilizam as restrições, inclusive Host; URL não ganha percent-encoding implícito.
`Session.RenderJSON` reutiliza o reconhecimento contextual: valores completos
recebem JSON tipado; conteúdo de strings usa representação escalar e escape JSON.
O body inline final é validado com `encoding/json` antes de `Execute`, também na borda
do runner para verificar o contrato do renderer. Offsets de erros mapeiam para
linha/coluna originais, inclusive CRLF, sem usar o comprimento renderizado.
O cenário não é mutado: requisição recebe clones de headers/body e somente o
resultado da substituição não recursiva. Não há body de diagnóstico.
Uma falha de renderização produz `RenderFailed`, posição localizada e callback
síncrono, mas **não** incrementa Started nem decrementa NotStarted.
Resultados anteriores e resumo permanecem disponíveis; próximas entradas param.

`template.Text` separa `Value` operacional de `Display` protegido por proveniência.
Substituições secretas são ocultas antes de parse/escapes da URL exibível, mesmo
em paths; `EntryResult.DisplayURL` alimenta `displayURL`, que ainda remove
userinfo/query/fragmento. O valor vazio nunca dispara substituição global de
texto vazio. Valores conhecidos e variantes escapadas também são removidos dos
streams. Isso não é detecção universal de credenciais ou codificações arbitrárias.
`--secret` não fornece valor, e a CLI proíbe credenciais reais em argumentos na
documentação: credenciais iniciais podem vir de referências explícitas ao ambiente
do processo (US-13); stdin permanece não implementado.

## Sessão e capturas

Cada `Run` pede uma nova `template.Session` ao `runner.Renderer`. `template.Context`
compartilha apenas as variáveis iniciais imutáveis, clonando as marcações de
segredo e criando um mapa separado de capturas. Não há mutação do cenário nem vazamento para o próximo Run.
O parser reconhece `[Captures]`, conserva `Capture.Position`/`QueryPosition` e usa
`query.Parse` para validar consultas JSONPath antes da rede. `Capture.Kind`
distingue `JSONPathCapture`, `HeaderCapture` e `RegexCapture` (body, US-25); header guarda o nome
literal em `Capture.Header`, validado pelas mesmas regras das assertions.
Resolução depende somente da resposta final da entrada executada.

`runner.Request.RetainBody` é verdadeiro quando há captura JSONPath/regex de body ou assertion
JSON/body; capturas e assertions exclusivamente de header não retêm body. Após transporte concluído, runner
compara status numérico e depois percorre headers/JSON na ordem escrita antes
de chamar `captureResponse(session, declarations, response)`, independentemente da ordem das seções. Header
anterior divergente prevalece sobre JSON inválido; JSON anterior inválido é erro. Sem expectativa, qualquer status permite
capturar sem inventar assertion. `query.Path.Select` devolve valor/found/erro seguro;
`null` difere de ausência, e números não passam por float64. O runner serializa
cada seleção com `json.Marshal`, preservando `json.Number` inclusive nas estruturas;
`nil` encontrado vira JSON `null`. O lote usa `template.Capture.Value` como
`json.RawMessage`. `Session.Publish` valida todo payload JSON e nomes/conflitos
antes de disponibilizá-lo atomicamente. Payload inválido é erro controlado de
captura, nunca fallback textual de `--var`. A sessão reutiliza `parseVariable`
somente após validação: strings são decodificadas, estruturas permanecem opacas,
valores são copiados sem alias do buffer do chamador. Strings recebidas são
serializadas como strings, sem reinterpretação de literais ou templates.
O renderer contextual existente preserva tipos completos/raiz e rejeita
arrays/objetos em texto antes do próximo envio; não há ampliação de JSONPath,
pacotes, dependências, goroutines ou contrato HTTP.

`CaptureFailed` não desfaz Completed, Validated ou WithoutValidation: a resposta
terminou e sua eventual comparação passou. `CaptureError` localiza declaração ou
consulta sem reproduzir JSON, valor selecionado ou causa bruta. Falha impede as
próximas entradas; callback síncrono e prioridade de ReportErr permanecem.
`EntryResult` não carrega o body da resposta nem os valores publicados.

A sessão registra proteção de valores JSON secretos assim que descobertos, inclusive
antes de uma eventual falha em outra declaração. Runner sanitiza DisplayURL antes
do callback. `invocationSession` combina `Session.Sanitize` com `executionProtection`,
que acumula variantes de redação deduplicadas em um mapa local a ExecuteContext,
sem guardar sessões ou capturas completas e sem sobreviver na instância original
do comando. Os writers combinam essa memória com `argumentProtection`/configuração.
`template.Session.ProtectionValues` fornece snapshot independente das variantes,
reutilizando `SecretVariants`/`Protector`, sem publicar valores no próximo contexto. Variantes para
capturas partem de JSON validado: `ProtectCapture(name, json.RawMessage)` reutiliza
`SecretVariants`/`Protector`, inclusive constituintes e escapes conhecidos de
arrays/objetos, sem disponibilizar propriedades como variáveis. A proteção também
precede falha de publicação/conflito; vazio não corrompe a saída. Não há confusão
entre string capturada e literal CLI. Não há nova fonte de credenciais iniciais nem detecção
universal de segredos ainda desconhecidos.

### Capturas de headers — US-22 / issue #42

`captureResponse` despacha explicitamente pelo `Capture.Kind`. JSONPath usa o
seletor existente; header usa `query.Header(response.Headers, declaration.Header)`,
primeira ocorrência e presença separada de vazio, sem `net/http` no runner.
`json.Marshal` converte a string selecionada em valor JSON explícito para o mesmo
lote tipado/`Session.Publish`, sem coerção de literais CLI. Qualquer consulta ou
colisão falha o lote inteiro, sem desfazer valores previamente publicados.
Variante desconhecida é erro explícito, nunca ignorada.

Antes da execução, declarações de captura de headers sensíveis são classificadas
com `sensitiveHeader` e `Session.MarkSecret`. `Context.NewSession` clona o mapa de
marcações com `maps.Clone`; marcas explícitas e dinâmicas acumulam sem alterar o
contexto inicial. Classificar não seleciona nem publica valores e não muda a
precedência: `pendingSecret` oculta operandos de assertions mesmo se a captura
falharia. Valores sensíveis recebidos passam pelo `ProtectValue` existente;
`ProtectCapture` preserva variantes JSON/Go/URL e proveniência para composições.
`safeEntry`, proteção de invocação e writers permanecem o único caminho de saída,
sem headers/capturas/body operacionais nos callbacks. Vazio não aciona redação
global; codificações arbitrárias não são detectadas.

O transporte de produção e `HTTPExecutor` não mudam: a resposta final é consumida
e fechada antes do retorno, mesmo com `RetainBody=false`. Read/close/timeout/cancel
impedem toda publicação e contagem de resposta completa. Redirects mantêm mesma
origem, um prazo e nenhum retry. Captura de `Set-Cookie` não instala cookie jar,
não transforma atributos nem sintetiza `Cookie` em requisições seguintes.

## Regex de headers — US-24 / issue #46

A fatia reutiliza US-07 e US-15, sem novos pacotes, interfaces, dependências ou
mudança de Go. `parser.parseHeaderAssertion` reconhece `assertion.HeaderMatches`;
`parseMatches` compartilha `jsonString`, compilação e classificação de erros com
`jsonpath matches`. O operando é uma única string JSON, não `assertionString`
(`strconv.Unquote`), preservando a gramática anterior dos outros operadores.
Erro estático em qualquer entrada rejeita todo o arquivo antes da rede, localizado
sem texto bruto de `regexp`/`regexp/syntax`. O padrão decodificado fica somente em
`HeaderAssertion.Expected`; `Compiled` pertence ao cenário imutável. Sem cache
ou compilação em execução, renderer ou resolução de placeholders.

`runner.checkHeader` consulta `query.Header` e despacha somente `HeaderMatches`
para `assertion.MatchString(declaration.Compiled, actual, present)`; os demais
operadores mantêm `assertion.Header`. A query continua intacta: primeira ocorrência
sem distinguir caixa, presença separada do texto vazio, sem juntar/dividir vírgulas.
Engine e semântica Go da US-15: busca parcial, âncoras/opções/Unicode, sem normalização.
Ausência sempre falha em matches, mesmo com padrão vazio. Não se serializa header
ou body para a comparação.

Fluxo inalterado: transporte retorna resposta **final consumida/fechada** → status
→ assertions na ordem escrita → capturas → resultado/callback → próximo envio.
`HTTP *` dispensa só comparação numérica. Header isolado não pede `RetainBody` nem
decode JSON; resposta incompleta, read/close/timeout/cancel não chega à avaliação.
Sem nova política de rede, prazo de regex, retenção, goroutine ou retry.

A falha usa `HeaderFailure`/`HeaderMismatch`, diagnóstico existente na CLI e códigos
0/1/2, sem reavaliação. Padrão percorre a mesma proteção do esperado literal:
ocultação integral em nomes sensíveis e captura secreta pendente; sanitização de
segredos conhecidos/derivados nos demais, antes de coleção/callback/streams.
`safeEntry` clona declaração e slice, zera `Compiled` (a regex retém fonte) e
preserva apenas esperado/metadados seguros, em sucesso e em qualquer falha.
Cenário/renderer reutilizáveis não compartilham proteção dinâmica entre sessões.
Falha de escrita/short write continua prioritária, sem próximo envio/reimpressão
ou resumo no writer que falhou; contagens/resultados anteriores são preservados.

Ajuda e exemplo `scenario_assert_header_matches.hurl` são exercitados pelo binário.
O harness diferencial existente usa `response_headers`, Hurl 8.0.0 obrigatório e
30 pares novos, sem modificar observações anteriores. Ver
[matriz](compatibility.md#regex-de-headers--us-24--issue-46) e
[evidência](validation.md#us-24--issue-46--história-0024).
Repetição/lista, interpolação, gramática e códigos permanecem diferenças explícitas.
Não há assertion regex de body/variável nem seleção de todas as ocorrências.
Extração de body/header é o recorte US-25 abaixo.

## Extração regex — US-25 / issue #48

`parser.Capture` acrescenta `Pattern`, `Compiled` (`*regexp.Regexp`) e
`PatternPosition`. `parseCaptureRegex` usa `jsonString`, compilação Go e
`unsupportedRegexp` da US-15: exatamente uma string JSON estática, sem renderer,
seletor de grupo ou pipeline. Todas as declarações são validadas antes da rede;
regex válida sem grupo permanece erro de execução. Posições indexam os bytes
originais do arquivo, inclusive LF/CRLF, não os escapes decodificados.

`query.Regex` usa `FindStringSubmatchIndex`: índices do primeiro grupo da primeira
correspondência distinguem grupo vazio de não participante. Não recompila nem
busca matches posteriores. Falhas retornam mensagens controladas sem texto,
padrão ou causa bruta. Em `captureResponse`, `RegexCapture` obtém texto via
`query.BodyText` somente quando alcançado; capturas de body da mesma resposta
reutilizam o texto local. `HeaderCapture` com `Compiled` aplica a regex à string
da primeira ocorrência, sem decoder nem JSON. O grupo vira string JSON no lote
existente; `Session.Publish` continua único e atômico após todas as seleções.

Marcação de origem sensível é feita antes de status/assertions; padrão sensível
é registrado para sanitização, sem avaliar a captura antecipadamente. O grupo
extraído passa imediatamente por `ProtectCapture`: proteger somente o header
inteiro não cobre seus fragmentos. Body usa as marcações explícitas existentes.
Uma captura posterior inválida ou colisão não desfaz essa proteção. `safeEntry`
remove capturas por completo, inclusive regex compilada e padrão, sem acumular
resposta nos resultados. Proteção da invocação continua entre arquivos; resolução
de capturas e marcas dinâmicas não atravessam sessões. Transporte/interfaces,
ordem, prazos, redirects, recursos e prioridade de escrita permanecem intactos.

## Headers e expectativa curinga — US-07

`Entry.ExpectedStatus` não nulo representa comparação numérica; `Entry.AnyStatus`
representa `HTTP *`; ambos ausentes mantêm execução sem expectativa. O parser
rejeita duplicações e `[Asserts]` sem qualquer declaração HTTP. Estados explícitos
permitem transições `[Captures]` ↔ `[Asserts]`, mantendo posições e rejeitando
seções vazias/duplicadas. `parser.HeaderAssertion` contém nome, operador, esperado
literal e `Position`; o renderer nunca recebe esse esperado como template.

`transport` converte somente os headers finais após consumir/fechar todo o body.
`net/http` canonicaliza nomes recebidos e mantém a ordem dos valores de cada nome,
inclusive caixas diferentes no wire. A conversão ordena somente nomes distintos,
nunca valores; não depende de iteração de mapa para escolher a primeira ocorrência,
não concatena nem separa vírgulas. `Response.Headers` usa dados Go próprios de
`query.HeaderValue`, sem expor `net/http.Header`. Assertions de headers isolados
não ligam `RetainBody`.

O fluxo é resposta completa → status numérico → assertions ordenadas
(`query.Header`/`assertion.Header` ou JSON) → capturas → callback. `HTTP *` produz `Wildcard`, conta em `WithoutValidation`,
não em `Validated`; continua distinto de `Unvalidated`. Falha de header produz
`HeaderMismatch`, `Result.Diverged=true` e `HeaderFailure`, não `Result.Err` de
transporte/captura. A CLI possui ramo próprio, sem acessar status esperado nulo.
A comparação numérica aprovada continua contada se um header falhar. Nenhuma
captura é avaliada/publicada após divergência; falha de escrita prevalece com 2.

Antes de renderizar a primeira entrada, a sessão registra os esperados sensíveis;
antes do callback, registra valores sensíveis de headers enviados/recebidos via
`Session.ProtectValue`, reutilizando `SecretVariants`/`Protector`. Esses valores
ficam na proteção local da execução, não como variáveis. `Authorization`,
`Proxy-Authorization`, `Cookie` e `Set-Cookie` têm esperado/recebido ocultos
integralmente; headers comuns passam pela proteção de segredos conhecidos.
`HeaderFailure` é sanitizado antes da formatação/coleção/callback. `safeEntry`
remove headers/body/BodyPlan da requisição do resultado, clona e sanitiza
assertions e metadados exibíveis sem mutar o cenário. Os writers existentes da
CLI também protegem resumos, nomes de arquivo e diagnósticos. Ausência mantém
representação própria, distinta de string vazia; strings usam quoting seguro.
Não se cria pacote report, interface, dependência ou mecanismo genérico de predicados.

O diferencial Hurl 8.0.0 classifica três divergências deliberadas: primeira
ocorrência em vez de lista para repetidos, esperado literal em vez de interpolado
e headers antes de capturas. Veja os pares `headers_*` em `compatibility.md`.

## Igualdade JSON e lista ordenada — US-08

`Entry.Assertions` passa a `[]parser.Assertion`, com variantes concretas
`Header *HeaderAssertion` e `JSON *JSONAssertion`. O parser produz exatamente uma
por item, preservando a ordem; consumidores rejeitam variante desconhecida em
vez de ignorá-la. `JSONAssertion` conserva consulta, `query.Path`, esperado JSON
decodificado e posições de declaração/consulta. Esperados nunca passam pelo
renderer: `query.Decode` exige exatamente um literal com UseNumber e `query.Parse`
valida consultas integralmente antes de retornar cenário. Não há mecanismo genérico
de predicados, novo pacote/interface de executor ou dependência.

`runner.checkAssertions` valida headers/JSON na ordem escrita após o status.
Decodifica a resposta **somente** ao alcançar a primeira assertion JSON, usando
`query.Decode`, e reutiliza o documento nessa entrada. `Path.SelectValue` separa
presença (inclusive null), ausência (propriedade/índice) e percurso incompatível
(erro seguro). O adapter `Path.Select` mantém o contrato anterior das capturas:
incompatibilidade conta como ausência para aquela API. Não há leitores no runner,
decode antecipado, retry nem documento acumulado nos resultados.

`assertion.JSONEqual` compara tipos concretos recursivamente; arrays em ordem e
objetos por chaves/conteúdo, sem considerar a ordem das chaves. `json.Number` é
normalizado em dígitos significativos com sinal e expoente decimal usando big.Int
apenas para o expoente. Zeros são normalizados. Não expande potências de dez nem
converte para float64: a memória depende do tamanho da representação, não do valor
do expoente. Validação do JSON permanece separada da comparação.

Divergência de valor/tipo ou ausência produz `JSONMismatch`, `JSONFailure` seguro
e `Result.Diverged` (1). JSON inválido/adicional ou percurso incompatível produz
`JSONFailed` e `*JSONError` controlado (2), com consulta/posição, sem causa bruta.
A CLI apresenta esses ramos sem reavaliar igualdade nem acessar ExpectedStatus
nulo em HTTP *. Completed e contagens de status continuam válidas mesmo se JSON
falhar; falha impede capturas e próximos envios. ReportErr continua prioritário (2).

`Session.IsSecret` expõe **somente** a marcação de um nome: `pendingSecret` inspeciona
as capturas da resposta sem resolver/consultar seus valores. Havendo uma marcada,
falhas de headers/JSON ocultam integralmente ambos os operandos, sem antecipar
qualquer erro de captura. `safeEntry` clona variantes, remove literais JSON,
consultas compiladas, capturas, headers, body e plano do callback/resultado;
metadados exibíveis, posição, consulta e valores de falha são sanitizados antes
da coleção. Headers esperados também ficam inteiramente ocultos nesse caso.
Sem captura pendente secreta, usa proteção de segredos conhecidos/derivados e
variantes escapadas existentes, além da ocultação integral de headers sensíveis.
A proteção é local à execução, não muda comparação/envio nem promete detectar
segredos arbitrários não marcados.

O diferencial reutiliza `response_json`/`response_headers`; não há mudança de rede
de produção. `headers_query_unsupported.json` muda intencionalmente de rejeição
para sucesso; os outros 78 pares antigos não mudam. Vinte e um novos pares
`assert_json_*` classificam tipos, precisão, estruturas, ausência, literais,
precedência e restrições frente ao Hurl 8.0.0; total 100. Veja `compatibility.md`.

## Estrutura, quantidade e limites — US-14

`parser.JSONAssertion` acrescenta `Operator assertion.JSONOperator` e `Count bool`.
O parser valida `exists` sem esperado, count com os cinco comparadores/quantidade
inteira não negativa e comparação direta ordenada com esperado `json.Number`.
Usa os mesmos `query.Parse`/`Decode`, posições e lista ordenada; nunca interpola
operandos. Construção desconhecida continua erro explícito antes da rede.
`query.Path.SelectValue` e a API das capturas não mudam. Não há nova interface,
pacote, dependência, I/O, goroutine ou alteração do transporte.

`assertion.EvaluateJSON` decide presença, aplicabilidade de count e comparação.
`exists` usa o indicador de presença, não o conteúdo (null presente existe).
Count transforma somente arrays em `json.Number` com seu tamanho; não devolve a
coleção no `JSONEvaluation`. `ValidCount` valida valor matematicamente inteiro
não negativo sem limite de máquina (1.0/1e3 também são inteiros). A comparação
exata compartilhada por igualdade/ordenação normaliza dígitos e expoente `big.Int`,
compara sinal e magnitude, depois alinha apenas os dígitos significativos. Não
expande potências: alocação limitada ao tamanho dos operandos, mesmo para expoentes
maiores que inteiro de máquina.

`runner.checkAssertions` mantém decode sob demanda e ordem status → lista →
capturas. Ausência, tipo selecionado incompatível e condição falsa produzem
`JSONMismatch` (1); JSON inválido/percurso incompatível mantêm `JSONFailed` (2).
`JSONFailure` acrescenta operador, count e detalhe de tipo controlado. Count exibe
a quantidade; tipo incompatível exibe apenas o nome do tipo, não seu conteúdo.
Os valores/consulta/localização são sanitizados antes do callback/coleção;
`pendingSecret` oculta inclusive contagens sem avaliar capturas. `safeEntry`
continua removendo `Expected` e `Path`; os novos campos são só metadados sem
operandos brutos. A CLI apenas formata operador/causa; não reavalia. `ReportErr`
continua prioritário e o cenário original é preservado.

Vinte novos pares `structure_*` reutilizam `response_json` e o harness existente,
sem modificar as 116 observações anteriores. Null em exists foi confirmado
contra o binário Hurl 8.0.0 antes de classificá-lo como compartilhado. Gramática,
pré-validação e códigos distintos constam de `compatibility.md`.

## Rede e recursos

`Execute` cria um único contexto filho com timeout efetivo (default de 10 segundos) e chama
`http.Transport.RoundTrip` diretamente. O loop de redirects é explícito, para
consumir e fechar também respostas intermediárias, verificar limite/origem antes
do próximo envio e manter o mesmo deadline. O teste observa os deadlines nas
requisições reais e comprova que não são renovados entre redirects.

O adapter cria cada request com o método recebido, headers explícitos e um reader
dos bytes do JSON. `Host` é convertido para `http.Request.Host`; headers repetidos
mantêm seus valores. O parser rejeita Host inválido/duplicado e headers explícitos
de enquadramento, evitando que net/http os ignore ou reinterprete silenciosamente.
301/302/303 convertem métodos diferentes de GET/HEAD em GET sem body, enquanto
307/308 criam outro reader dos mesmos bytes. Os headers declarados são mantidos;
a próxima origem e o limite são verificados **antes** do envio. Isso preserva as
políticas US-01 sem delegar redirects/retries a `http.Client`.

O parser delega o reconhecimento JSON contextual a `internal/template/json.go`,
que percorre estrutura, strings e escapes e valida o resultado mascarado com
`encoding/json`. Não cria um segundo mecanismo no parser. Lê o arquivo completo
antes da rede, calcula posições sobre os bytes originais e conserva whitespace,
escapes e CRLF. Bodies estáticos não são reserializados. Arrays/escalares, placeholder
raiz e instruções dentro de strings não se confundem com delimitadores externos.
Nenhum erro bruto de JSON/transporte é concatenado ao diagnóstico.

A conexão não é reutilizada (`DisableKeepAlives`) e HTTP/2 não é habilitado
(`ForceAttemptHTTP2=false`, `TLSNextProto` vazio não nulo). A decisão sacrifica
pooling para impedir os caminhos de retry implícito de GETs em conexões
reutilizadas e de streams HTTP/2. O teste de warm-up + falha encerra a segunda
conexão e comprova ausência de replay e do GET seguinte.

`io.Copy(io.Discard, body)` consome intermediárias e respostas sem consultas que precisem do body em
streaming. `io.ReadAll` retém apenas a resposta final quando RetainBody foi pedido,
no mesmo contexto/deadline. O body é fechado em sucesso ou erro; falhas de leitura
ou fechamento continuam sendo transporte, sem publicação parcial de capturas.
O retorno só ocorre depois desse consumo e sem cancelamento pendente. Runner
conhece somente bytes fechados de resposta, nunca leitores ou tipos net/http.
Após avaliação, os bytes/documento não são acumulados no resultado. Memória adicional
acompanha a resposta final e suas estruturas nas entradas com captura JSONPath/assertion JSON/body e
os valores/proteções da sessão; não há retenção de resposta em `EntryResult`.

Causas sensíveis são preservadas para `errors.Is/As`, mas jamais concatenadas à
mensagem exibida. A CLI usa categoria controlada ou diagnóstico genérico seguro;
URL inválida não é renderizada. `displayURL` centraliza remoção de userinfo/query/
fragmento. Coleções de headers e bodies não são apresentadas; somente valores
comuns sanitizados podem aparecer no diagnóstico de assertion. Valores de
Authorization/Proxy-Authorization/Cookie/Set-Cookie e credenciais do JSON ficam
fora de ambos os streams. stdout e stderr têm papéis distintos e falhas de escrita
geram código 2.

## Suítes sequenciais — US-16

`cli.arguments.files` aceita caminhos múltiplos e `failFast` reconhece `--fail-fast`
sem valor. A preparação de configuração permanece única por invocação, antes de
qualquer HTTP. `suite.NewWithProtection` recebe o runner montado e somente a função
de proteção da invocação; não conhece configuração ou transporte concreto.

`Suite.Discover` inspeciona componentes explícitos antes de limpar `.`/`..` para
não aceitar `link/../arquivo`. Usa `Lstat`, percurso recursivo sem seguir links,
`ReadDir` e abertura/fechamento efetivos de arquivos regulares. Ordena candidatos
por `filepath.Rel(cwd, absoluto)` com `/`, com fallback absoluto entre unidades;
deduplica via `os.SameFile`, sem inferir caixa pelo SO. Cada diretório explícito
é verificado antes da deduplicação global. Falha descarta o plano inteiro, não
executa prefixo. Não é sandbox contra alterações concorrentes do filesystem.
Os pontos `open`, `lstat`, `readDir` e `now` são privados e por instância, sem
interface pública genérica nem relógio global.

Na vez do arquivo, `RunFile`/`load` abrem, verificam tipo, chamam `parser.Parse`,
fecham antes de HTTP e preservam causa de leitura/fechamento sem texto bruto.
`Scenario.OriginDir` é absoluto e separado do nome exibível normalizado. Erro
estrutural/carga não retorna cenário parcial; parsing não antecipa resolução de
capturas. `Runner.RunWithOptions` recebe opções por valor e pede sessão nova à
factory existente; variáveis iniciais permanecem imutáveis e o transporte continua
sem cookie jar. Payloads não mudam de raiz conforme configuração ou arquivo seguinte.

`Suite.Run` classifica erro, divergência e sucesso completo sem reavaliar assertions
nem usar `Summary.Validated` como prova de sucesso. Guarda `FileResult` com nome
protegido, estado, duração, contagens e indicador `Known`: nunca cenário, body,
response, sessão completa ou runner.Result. O runner.Result e erro de carga são
somente entradas transitórias do callback síncrono de apresentação. Erros expõem
mensagem controlada/protegida e causa via Unwrap. O código agregado usa 2 > 1 > 0.

Sem fail-fast, erro local continua; com a flag, posteriores têm `Skipped` e
contagens desconhecidas, sem carga/parsing só para contar. Cancelamento global
ou qualquer falha de apresentação interrompe sempre, mesmo sem fail-fast; timeout
HTTP isolado continua local. Relógio por instância mede durações não negativas.
Writers da invocação lembram a primeira falha/short write para não tentar o mesmo
writer novamente. Callback de entrada falhou: não há callback/resumo de arquivo
nem agregado posterior. Arquivos restantes são registrados compactamente como
não executados sem novos envios. Proteção antecede coleção/callback, inclusive
erros de parsing e nomes posteriores; novas variantes também protegem os nomes
retidos até o retorno. Invocação seguinte não herda opções, variantes ou resultados.

## Testes e evidência

- `internal/parser`: readers em memória, fronteiras/bytes JSON, headers, URLs,
  posições, falha de leitura com causa preservada, corpus inválido/não suportado e
  `FuzzParse` com seeds de métodos, JSON e headers.
- `internal/template`: tipos, precisão, nomes/conflitos, vazio/`=`, não recursão,
  JSON contextual/escapes/limites, variantes de segredos e origem; `FuzzRender`
  ampliado e novo `FuzzRenderJSON` (invariantes de JSON válido e ausência de mutação).
- `internal/assertion` / `internal/runner`: table-driven; fake HTTP sem rede,
  ausência de chamadas posteriores, resumo e contexto propagado.
- `internal/transport`: httptest HTTP/TLS, contadores de acesso e conexões,
  redirects, prazos configurados menores/maiores que dez segundos, body incompleto e cancelamento.
- `internal/cli`: root command com streams/args injetados; sem executar binário;
  quarenta entradas verificam resultado visível antes do próximo GET e falhas de
  escrita comprovam interrupção imediata e prioridade do exit code 2.
- `tests/e2e`: build do binário sem CGO uma vez por suíte; CommandContext, servidor
  local, streams, exit status, Ctrl+C real Linux/macOS (executado apenas Linux),
  timeout público real de 10 segundos e PTY Linux sem dependência C. Um pipe de
  stdout comprova o primeiro resultado já visível enquanto o servidor mantém a
  segunda requisição bloqueada, em sucesso, divergência e body incompleto.
- `tests/e2e/compatibility_test.go`: alvo explícito com tag `compatibility`, Hurl
  8.0.0 exigido e 100 pares/observações versionados, incluindo bytes/métodos/headers
  efetivamente recebidos no corpus US-02 e variáveis fornecidas às duas ferramentas
  sem editar cenários nos corpora US-03/US-04 (com RequestURI para verificar encoding).
  `variables_body` muda intencionalmente de rejeição para sucesso; seis novos pares
  distinguem semântica compartilhada e extensões de tipagem CLI. Nove pares US-05
  `captures_*` registram JSON retornado pelo harness e token efetivamente recebido
  em Authorization, além das diferenças de nomes, sobrescrita, tipos e ausência
  de expectativa. A US-06 acrescenta oito pares `captures_typed_*` para valores
  completos/raiz, índices, boolean/null, precisão e rejeições textuais; apenas
  `captures_non_string` muda de rejeição para sucesso entre os 50 anteriores.
  Os outros 49 pares conservam suas observações. A US-07 acrescenta 21 pares
  `headers_*` e respostas de headers repetidos/vazios no harness; os 58 pares
  anteriores permanecem inalterados naquela história. A US-08 acrescenta 21 pares
  `assert_json_*` e amplia intencionalmente `headers_query_unsupported`, mantendo
  os demais 78 antigos. Sem skip em versão ausente/errada.

Saída é deliberadamente sempre plain-text: os testes usam streams, arquivo, pipe e
PTY real para verificar o contrato, sem criar detector de TTY ou dependência
apenas para um formato que não muda. Consulte `validation.md` para evidências e
bloqueios; cross-build não é aceite nativo.
