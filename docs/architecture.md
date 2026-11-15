# Arquitetura implementada — US-01 a US-12 e US-20

Referência de aceite: issues GitHub #4 (US-01), #6 (US-02), #11 (US-03), #13 (US-04), #14 (US-05), #16 (US-06), #18 (US-07), #20 (US-08), #22 (US-09), #24 (US-20) e #25 (US-10), #27 (US-11) e #29 (US-12),
e plano v4 referenciado nelas.
Implementação independente baseada em requisitos/documentação pública, sem
copiar, adaptar ou traduzir código-fonte Hurl.

## Fluxo e arquivos reais

```text
cmd/httpgo/main.go
  → internal/cli/command.go + arguments.go (flags/contexto novo e proteção)
    → internal/config/config.go + value.go + compose.go + environment.go (YAML seguro e composição)
    → internal/parser/parser.go + capture.go + assertion.go + multipart.go (arquivo inteiro)
      → internal/query/jsonpath.go (validar consultas antes da rede)
    → internal/runner/runner.go + render.go (somente cenário validado)
      → internal/template/template.go + json.go (valores tipados, texto/JSON contextual)
      → parser.ValidateURL/ValidateHeader e validação JSON final
      → internal/runner/file.go (leitura integral e fechamento de body externo)
      → internal/runner/render.go (JSON externo opt-in com a mesma sessão)
      → internal/runner/multipart.go + multipart_mime.go (fonte aberta e textos preparados)
      → internal/transport/http.go + multipart.go (via HTTPExecutor)
      → fechamento da fonte multipart antes da avaliação/apresentação
      → internal/assertion/status.go (somente expectativa numérica)
      → internal/runner/assertions.go (headers/JSON na ordem escrita, antes de capturas)
        → internal/query/header.go + internal/assertion/header.go
        → internal/query/jsonpath.go + internal/assertion/json.go (sob demanda)
      → internal/query/jsonpath.go (selecionar JSON da resposta final)
      → internal/runner/capture.go (serializar seleção JSON sem float64)
      → internal/template/session.go (proteger e publicar JSON tipado atomicamente)
      → apresentação segura de cada resultado na CLI (callback síncrono)
    → resumo final na CLI
```

O diagrama é fluxo de execução; `runner` **não importa** o adapter HTTP.
Não há framework de CLI, pacote genérico de tipos, domínio rico ou novos pacotes
reservados para capacidades futuras. A US-11 acrescenta somente `internal/config`
e a dependência YAML mantida `go.yaml.in/yaml/v3` **v3.0.6**, isolada nesse pacote.
O restante usa biblioteca padrão; `go.mod` preserva módulo
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
| `assertion.Status`, `HeaderOperator`, `assertion.Header`, `assertion.JSONEqual` | Comparação de status/headers e igualdade JSON tipada recursiva, sem coerção nem float64; não conhece runner, HTTP ou apresentação |
| `runner.HeaderFailure`, `JSONFailure`, `JSONError` | Divergências e erros de avaliação localizados; nome/consulta, esperado/recebido sanitizados e presença explícita; sem coleção/body/documento bruto |
| `transport.NetHTTPExecutor`, `Error`, `ErrorKind` | HTTP real, prazo, redirects, consumo/fechamento e classificação segura com causa preservada via Unwrap |
| `cli.Command`, `BuildInfo` | Argumentos e streams (`SetArgs/SetIn/SetOut/SetErr`), leitura/fechamento antes da rede, saída segura, resumo e exit code; não reavalia assertion |
| `main.run` | Monta dependências, `signal.NotifyContext`, streams do processo; seus defers terminam antes do único os.Exit de produção em main |

`AGENTS.md` registra a exceção autorizada para `transport` importar **os tipos**
`runner.Request/Response`, implementando o contrato consumer-owned sem camada
adicional nem ciclo. Testes de integração podem montar `runner.New` com o adapter.
Somente o código de produção em `transport` importa `net/http`.

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
referências de processo e documento único. O schema aceita somente version 1,
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

Configuração inválida termina com 2 antes de parsing/HTTP. Não há
`HTTPGO_*`, referências ao ambiente do processo, stdin ou hooks; `${NOME}` é
rejeitado mesmo quando o nome teria override. YAML é extensão HTTPGo, com testes
próprios separados do diferencial Hurl e no máximo 1 MiB lido.

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
adicionada. `${NOME}` continua rejeitado por fonte, sem consultar o processo.

Testes `TestEnvironment*` em config/CLI/E2E cobrem GO-141–GO-160; extensões
continuam separadas do corpus Hurl. Evidências por cenário em `validation.md`.

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

Ao terminar ou interromper a execução, `renderSummary` escreve o resumo uma única
vez e apresenta o diagnóstico/exit code. Resultados já emitidos não são reimpressos.
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
documentação: não há entrada segura por ambiente/stdin nesta entrega.

## Sessão e capturas

Cada `Run` pede uma nova `template.Session` ao `runner.Renderer`. `template.Context`
compartilha apenas as variáveis iniciais e marcações imutáveis, criando um mapa
separado de capturas. Não há mutação do cenário nem vazamento para o próximo Run.
O parser reconhece `[Captures]`, conserva `Capture.Position`/`QueryPosition` e usa
`query.Parse` para validar toda consulta antes da rede; resolução depende somente
da resposta da entrada executada.

`runner.Request.RetainBody` é verdadeiro em entradas com capturas **ou** assertions
JSON; headers isolados continuam sem retenção. Após transporte concluído, runner
compara status numérico e depois percorre headers/JSON na ordem escrita antes
de chamar `captureResponse`, independentemente da ordem das seções. Header
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
do callback; os writers da CLI combinam `argumentProtection` e `Session.Sanitize`
antes de escrever resultados/diagnósticos/resumos. `executionProtection` é local a
ExecuteContext, sem sobreviver na instância do comando original. Variantes para
capturas partem de JSON validado: `ProtectCapture(name, json.RawMessage)` reutiliza
`SecretVariants`/`Protector`, inclusive constituintes e escapes conhecidos de
arrays/objetos, sem disponibilizar propriedades como variáveis. A proteção também
precede falha de publicação/conflito; vazio não corrompe a saída. Não há confusão
entre string capturada e literal CLI. Não há nova fonte de credenciais iniciais nem detecção
universal de segredos ainda desconhecidos.

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

`io.Copy(io.Discard, body)` consome intermediárias e respostas sem capturas/JSON em
streaming. `io.ReadAll` retém apenas a resposta final quando RetainBody foi pedido,
no mesmo contexto/deadline. O body é fechado em sucesso ou erro; falhas de leitura
ou fechamento continuam sendo transporte, sem publicação parcial de capturas.
O retorno só ocorre depois desse consumo e sem cancelamento pendente. Runner
conhece somente bytes fechados de resposta, nunca leitores ou tipos net/http.
Após avaliação, os bytes/documento não são acumulados no resultado. Memória adicional
acompanha a resposta final e suas estruturas nas entradas com captura/JSON e
os valores/proteções da sessão; não há retenção de resposta em `EntryResult`.

Causas sensíveis são preservadas para `errors.Is/As`, mas jamais concatenadas à
mensagem exibida. A CLI usa categoria controlada ou diagnóstico genérico seguro;
URL inválida não é renderizada. `displayURL` centraliza remoção de userinfo/query/
fragmento. Coleções de headers e bodies não são apresentadas; somente valores
comuns sanitizados podem aparecer no diagnóstico de assertion. Valores de
Authorization/Proxy-Authorization/Cookie/Set-Cookie e credenciais do JSON ficam
fora de ambos os streams. stdout e stderr têm papéis distintos e falhas de escrita
geram código 2.

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
