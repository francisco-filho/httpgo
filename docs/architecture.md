# Arquitetura implementada — US-01 a US-07

Referência de aceite: issues GitHub #4 (US-01), #6 (US-02), #11 (US-03), #13 (US-04), #14 (US-05), #16 (US-06) e #18 (US-07),
e plano v4 referenciado nelas.
Implementação independente baseada em requisitos/documentação pública, sem
copiar, adaptar ou traduzir código-fonte Hurl.

## Fluxo e arquivos reais

```text
cmd/httpgo/main.go
  → internal/cli/command.go + arguments.go (flags/contexto novo e proteção)
    → internal/parser/parser.go + capture.go + assertion.go (arquivo inteiro)
      → internal/query/jsonpath.go (validar consultas antes da rede)
    → internal/runner/runner.go + render.go (somente cenário validado)
      → internal/template/template.go + json.go (valores tipados, texto/JSON contextual)
      → parser.ValidateURL/ValidateHeader e validação JSON final
      → internal/transport/http.go (via HTTPExecutor)
      → internal/assertion/status.go (somente expectativa numérica)
      → internal/query/header.go + internal/assertion/header.go (antes de capturas)
      → internal/query/jsonpath.go (selecionar JSON da resposta final)
      → internal/runner/capture.go (serializar seleção JSON sem float64)
      → internal/template/session.go (proteger e publicar JSON tipado atomicamente)
      → apresentação segura de cada resultado na CLI (callback síncrono)
    → resumo final na CLI
```

O diagrama é fluxo de execução; `runner` **não importa** o adapter HTTP.
Não há framework de CLI, pacote genérico de tipos, domínio rico ou novos pacotes
reservados para capacidades futuras. Toda a implementação usa biblioteca padrão;
`go.mod` preserva módulo `capimgrosso.com/httpgo` e Go `1.26.10`.

| Pacote/tipos | Responsabilidade |
|---|---|
| `parser.Position`, `Header`, `HeaderAssertion`, `Capture`, `Entry`, `Scenario`, `Error` | Modelo ordenado com método, URL, headers, body, capturas e posições de declaração/consulta; estados de requisição, expectativa e capturas; validação completa sem cenário parcial |
| `runner.Request`, `Response`, `HTTPExecutor` | Contrato mínimo definido no consumidor, sem net/http: `Execute(context.Context, Request) (Response, error)`; Request carrega Method, URL, Headers (`[]parser.Header`), Body (`[]byte`) e RetainBody; Response retorna Status, Headers (`[]query.HeaderValue`, ocorrências ordenadas) e Body final retido somente quando solicitado |
| `runner.Runner`, `EntryResult`, `Outcome`, `Summary`, `Result` | Ordem, contexto antes de cada chamada, comparação delegada e interrupção; resultado estruturado e notificação síncrona por entrada, sem escrever em streams nem chamar os.Exit |
| `template.Context`, `Text`, `Reference`, `Error` | Contexto tipado isolado, nomes exatos/conflitos, offsets, substituição não recursiva e proveniência; não importa parser/runner/CLI/transporte |
| `template.JSONPlan`, `JSONReference` | Reconhecimento de strings/escapes/valores e chaves estáticas, referências com intervalos originais, limite exato do body; compartilhado por parser e renderer |
| `template.SecretVariants`, `Protector` | Proteção de valores brutos, decodificados e escapes conhecidos, inclusive antes de validar contexto; sem substituição de texto vazio |
| `runner.Renderer`, `RenderError`, `CaptureError` | Factory consumer-owned `NewSession() template.Session` por Run; erros pré-envio e de captura com categorias/origens distintas; clones preservam cenário |
| `template.Session`, `Capture`, `ConflictError`, `CaptureValueError` | Renderização e publicação atômica de JSON validado (`Capture.Value` é `json.RawMessage`); erro de payload/conflito indexa o lote sem expor conteúdo; variáveis iniciais imutáveis, cópia dos valores, capturas separadas e proteção dos valores descobertos |
| `query.Path`, `query.Error` | Pacote folha sem interface: validar JSONPath limitado e selecionar valor com `UseNumber`, distinguindo ausência de null; valores JSON de qualquer tipo e números exatos |
| `query.HeaderValue`, `query.Header` | Ocorrências nome/valor sem net/http; seleção pura da primeira, com presença distinta de vazio e nome case-insensitive |
| `assertion.Status`, `HeaderOperator`, `assertion.Header` | Igualdade numérica e comparação literal de headers (`==`, `contains`, `exists`); não conhece runner, HTTP ou apresentação |
| `runner.HeaderFailure` | Falha de assertion localizada com nome/operador, esperado/recebido já sanitizados e presença explícita; sem coleção bruta |
| `transport.NetHTTPExecutor`, `Error`, `ErrorKind` | HTTP real, prazo, redirects, consumo/fechamento e classificação segura com causa preservada via Unwrap |
| `cli.Command`, `BuildInfo` | Argumentos e streams (`SetArgs/SetIn/SetOut/SetErr`), leitura/fechamento antes da rede, saída segura, resumo e exit code; não reavalia assertion |
| `main.run` | Monta dependências, `signal.NotifyContext`, streams do processo; seus defers terminam antes do único os.Exit de produção em main |

`AGENTS.md` registra a exceção autorizada para `transport` importar **os tipos**
`runner.Request/Response`, implementando o contrato consumer-owned sem camada
adicional nem ciclo. Testes de integração podem montar `runner.New` com o adapter.
Somente o código de produção em `transport` importa `net/http`.

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
ou `*runner.CaptureError`;
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
O body final é validado com `encoding/json` antes de `Execute`, também na borda
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

`runner.Request.RetainBody` é verdadeiro apenas em entradas com capturas.
Após transporte concluído, runner compara status numérico e depois headers antes
de chamar `captureResponse`, independentemente da ordem das seções.
Divergência prevalece sobre JSON inválido. Sem expectativa, qualquer status permite
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
`query.HeaderValue`, sem expor `net/http.Header`. Assertions não ligam `RetainBody`.

O fluxo é resposta completa → status numérico → `query.Header` → `assertion.Header`
→ capturas → callback. `HTTP *` produz `Wildcard`, conta em `WithoutValidation`,
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

## Rede e recursos

`Execute` cria um único contexto filho com timeout de 10 segundos e chama
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

`io.Copy(io.Discard, body)` consome intermediárias e respostas sem capturas em
streaming. `io.ReadAll` retém apenas a resposta final quando RetainBody foi pedido,
no mesmo contexto/deadline. O body é fechado em sucesso ou erro; falhas de leitura
ou fechamento continuam sendo transporte, sem publicação parcial de capturas.
O retorno só ocorre depois desse consumo e sem cancelamento pendente. Runner
conhece somente bytes fechados de resposta, nunca leitores ou tipos net/http.
Após avaliação, os bytes não são acumulados no resultado. Memória adicional acompanha a resposta final retida nas entradas com captura e
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
  redirects, prazo menor **somente em testes internos**, body incompleto e cancelamento.
- `internal/cli`: root command com streams/args injetados; sem executar binário;
  quarenta entradas verificam resultado visível antes do próximo GET e falhas de
  escrita comprovam interrupção imediata e prioridade do exit code 2.
- `tests/e2e`: build do binário sem CGO uma vez por suíte; CommandContext, servidor
  local, streams, exit status, Ctrl+C real Linux/macOS (executado apenas Linux),
  timeout público real de 10 segundos e PTY Linux sem dependência C. Um pipe de
  stdout comprova o primeiro resultado já visível enquanto o servidor mantém a
  segunda requisição bloqueada, em sucesso, divergência e body incompleto.
- `tests/e2e/compatibility_test.go`: alvo explícito com tag `compatibility`, Hurl
  8.0.0 exigido e 79 pares/observações versionados, incluindo bytes/métodos/headers
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
  anteriores permanecem inalterados. Sem skip em versão ausente/errada.

Saída é deliberadamente sempre plain-text: os testes usam streams, arquivo, pipe e
PTY real para verificar o contrato, sem criar detector de TTY ou dependência
apenas para um formato que não muda. Consulte `validation.md` para evidências e
bloqueios; cross-build não é aceite nativo.
