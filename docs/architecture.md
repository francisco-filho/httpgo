# Arquitetura implementada — história #4

Referência de aceite: issue GitHub #4 (US-01) e plano v4 referenciado nela.
Implementação independente baseada em requisitos/documentação pública, sem
copiar, adaptar ou traduzir código-fonte Hurl.

## Fluxo e arquivos reais

```text
cmd/httpgo/main.go
  → internal/cli/command.go
    → internal/parser/parser.go (arquivo inteiro)
    → internal/runner/runner.go (somente cenário validado)
      → internal/transport/http.go (via HTTPExecutor)
      → internal/assertion/status.go
      → apresentação segura de cada resultado na CLI (callback síncrono)
    → resumo final na CLI
```

O diagrama é fluxo de execução; `runner` **não importa** o adapter HTTP.
Não há framework de CLI, pacote genérico de tipos, domínio rico ou novos pacotes
reservados para capacidades futuras. Toda a implementação usa biblioteca padrão;
`go.mod` preserva módulo `capimgrosso.com/httpgo` e Go `1.26.10`.

| Pacote/tipos | Responsabilidade |
|---|---|
| `parser.Position`, `Entry`, `Scenario`, `Error` | Modelo ordenado e posições de GET/expectativa; parsing por estado (última entrada disponível/expectativa já definida); validação completa; zero cenário parcial em erro |
| `runner.Request`, `Response`, `HTTPExecutor` | Contrato mínimo definido no consumidor, sem net/http: `Execute(context.Context, Request) (Response, error)` |
| `runner.Runner`, `EntryResult`, `Outcome`, `Summary`, `Result` | Ordem, contexto antes de cada chamada, comparação delegada e interrupção; resultado estruturado e notificação síncrona por entrada, sem escrever em streams nem chamar os.Exit |
| `assertion.Status` | Somente igualdade esperado/recebido; não conhece runner, HTTP ou apresentação |
| `transport.NetHTTPExecutor`, `Error`, `ErrorKind` | HTTP real, prazo, redirects, consumo/fechamento e classificação segura com causa preservada via Unwrap |
| `cli.Command`, `BuildInfo` | Argumentos e streams (`SetArgs/SetIn/SetOut/SetErr`), leitura/fechamento antes da rede, saída segura, resumo e exit code; não reavalia assertion |
| `main.run` | Monta dependências, `signal.NotifyContext`, streams do processo; seus defers terminam antes do único os.Exit de produção em main |

`AGENTS.md` registra a exceção autorizada para `transport` importar **os tipos**
`runner.Request/Response`, implementando o contrato consumer-owned sem camada
adicional nem ciclo. Testes de integração podem montar `runner.New` com o adapter.
Somente o código de produção em `transport` importa `net/http`.

## Apresentação incremental

A CLI monta `runner.New(executor, c.renderEntry)`. Após classificar cada resposta
ou falha, o runner chama o callback **sincronamente, antes de iniciar a próxima
entrada**. `renderEntry` escreve uma linha sanitizada, sem reavaliar assertions.
Sucessos, inclusive sem expectativa, começam com `GET`; arquivo/linha/coluna
prefixam somente divergências e falhas. O resumo continua identificando o arquivo.
Não há goroutine, fila de eventos ou espera pelo fim do arquivo para apresentar
um resultado. Callback `nil` permite coletar resultados sem apresentação.

Ao terminar ou interromper a execução, `renderSummary` escreve o resumo uma única
vez e apresenta o diagnóstico/exit code. Resultados já emitidos não são reimpressos.
O resultado estruturado e suas contagens continuam disponíveis no retorno do runner.

Se o callback falhar, `Result.ReportErr` preserva a causa e o runner não inicia o
GET seguinte. A CLI diagnostica a falha de escrita em stderr com texto controlado,
retorna 2 (também se houve divergência) e não tenta escrever o resumo no stdout
que falhou. `Result.Err` continua reservado a transporte/cancelamento.

## Rede e recursos

`Execute` cria um único contexto filho com timeout de 10 segundos e chama
`http.Transport.RoundTrip` diretamente. O loop de redirects é explícito, para
consumir e fechar também respostas intermediárias, verificar limite/origem antes
do próximo envio e manter o mesmo deadline. O teste observa os deadlines nas
requisições reais e comprova que não são renovados entre redirects.

A conexão não é reutilizada (`DisableKeepAlives`) e HTTP/2 não é habilitado
(`ForceAttemptHTTP2=false`, `TLSNextProto` vazio não nulo). A decisão sacrifica
pooling para impedir os caminhos de retry implícito de GETs em conexões
reutilizadas e de streams HTTP/2. O teste de warm-up + falha encerra a segunda
conexão e comprova ausência de replay e do GET seguinte.

`io.Copy(io.Discard, body)` faz consumo em streaming; o body é fechado em sucesso
ou erro. O retorno de status só ocorre depois desse consumo e sem cancelamento
pendente. O runner não conhece response body ou tipos net/http.

Causas sensíveis são preservadas para `errors.Is/As`, mas jamais concatenadas à
mensagem exibida. A CLI usa categoria controlada ou diagnóstico genérico seguro;
URL inválida não é renderizada. `displayURL` centraliza remoção de userinfo/query/
fragmento. stdout e stderr têm papéis distintos e falhas de escrita geram código 2.

## Testes e evidência

- `internal/parser`: readers em memória, fronteiras, URLs, posições, falha de
  leitura com causa preservada e fixtures de construções não suportadas.
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
  8.0.0 exigido e corpus/observações versionados. Não há skip em versão ausente/errada.

Saída é deliberadamente sempre plain-text: os testes usam streams, arquivo, pipe e
PTY real para verificar o contrato, sem criar detector de TTY ou dependência
apenas para um formato que não muda. Consulte `validation.md` para evidências e
bloqueios; cross-build não é aceite nativo.
