# httpgo

CLI independente em Go para executar um **subconjunto documentado** de arquivos
`.hurl`. Implementação própria, sem reutilizar código-fonte do Hurl.

## Compartilhar resultados HTML offline (US-35 / issue #72)

```bash
httpgo test tests/ --fail-fast --output html --output-file target/report.html
```

Funciona para arquivo único ou suíte. `--output html` exige destino antes de
qualquer HTTP; stdout fica vazio e stderr informa o caminho final somente após
escrita/fechamento. `target/report.html` é um **nome-base**: gera, por exemplo,
`target/report-20261117T193045123Z.html` (UTC), com `-1`, `-2` etc. na primeira
colisão livre, sem sobrescrever relatórios anteriores. Cria pais quando possível.

HTML único, autossuficiente e legível offline, com tema somente claro, resumo,
estados textuais por arquivo, detalhes seguros por requisição e duração lógica
incluindo preparação/tentativas/esperas, sem reenvios para apresentação. Arquivos
não analisados têm contagens desconhecidas; ausência de assertion não é validação.
Segredos conhecidos/marcados/derivados são protegidos e textos escapados; sem
bodies/headers integrais. Não detecta todo segredo arbitrário não marcado.

Se health passa, products recebe 503 em vez de 200 e profile não inicia pelo
fail-fast, o relatório mostra um sucesso, uma falha de assertion e um não executado,
com **código 1**. **0/1/2** permanecem; geração ou confirmação falha retorna **2**,
sem anúncio falso. Erro global antes da execução não gera resultados fictícios;
escrita falha pode deixar arquivo parcial. JUnit mantém sua política de substituição.

[Demonstração versionada](testdata/scenarios/html_suite/),
[contrato, limites de sigilo e exemplos](docs/html.md).
Não há tema escuro, HTML/JUnit simultâneos ou compatibilidade anunciada com flags
HTML do Hurl. YAML e interpolação externa continuam extensões HTTPGo.

## Validar e renovar capturas na própria resposta (US-34 / issue #70)

**Mudança deliberada de contrato, padrão e sem nova flag:** resposta completa →
status esperado → todas as capturas → assertions na ordem escrita → confirmação.
Assertions enxergam o lote atual mesmo se `[Asserts]` vier antes de `[Captures]`.

```hurl
GET {{base_url}}/jobs/current
HTTP 200
[Captures]
job_id: jsonpath "$.selectedId"
[Asserts]
jsonpath "$.jobs[?(@.id=='{{job_id}}')].status" == "READY"

GET {{base_url}}/jobs/{{job_id}}/successor
HTTP 200
[Captures]
job_id: jsonpath "$.id"
[Asserts]
jsonpath "$.status" == "READY"

GET {{base_url}}/jobs/{{job_id}}
HTTP 200
```

```bash
httpgo test testdata/scenarios/scenario_capture_renewal.hurl --render-asserts --var base_url=http://localhost:8080 --var job_id=inicial
```

`/jobs/current` deve devolver `selectedId: "job-a"` e `jobs` contendo esse ID
com status `READY`; `/jobs/job-a/successor`, `{"id":"job-b","status":"READY"}`.
O fluxo consulta `/jobs/job-b` e termina com **0**. Selecionado não pronto retorna
**1**, sem a segunda requisição. O exemplo exige esses endpoints, não os fornece.

- Captura substitui nome **exatamente igual** inicial ou confirmado, inclusive
  mudando tipo, sem perda de precisão. Não altera configuração ou outros arquivos.
  Duplicação no lote e conflitos ancestral/descendente continuam **erro 2**;
  objetos não criam variáveis para suas propriedades.
- Consultas de captura leem somente iniciais/confirmados anteriores, nunca novas
  capturas do lote. Assertions usam o novo valor. Para comparar antigo e novo,
  use nomes distintos. Esperados e operandos de filtro só interpolam com
  `--render-asserts`; o perfil literal não muda.
- Erro de captura precede assertions. Status divergente impede capturas. Falha de
  captura/avaliação, divergência, timeout ou cancelamento antes da confirmação
  descarta o lote inteiro. Efeitos já feitos na API não são desfeitos.
- Polling mantém os valores confirmados antes da entrada em todos os reenvios;
  somente a tentativa aprovada confirma. Erro **2** não é retentado; divergência
  **1** continua elegível dentro do orçamento existente.
- Marcas/valores secretos antigos, novos e derivados permanecem protegidos mesmo
  após descarte e entre arquivos da invocação, sem compartilhar valores resolvíveis.
  Segredos arbitrários não marcados/transformações desconhecidas não são detectados.
  Credenciais iniciais vêm do ambiente, nunca de flags.
- Não habilita query `variable`, encadeamento de capturas do mesmo lote, novas
  políticas de retry ou de entrada de credenciais.

[Contrato](docs/subset.md#renovação-transacional-de-capturas--us-34--issue-70),
[arquitetura](docs/architecture.md#renovação-transacional-de-capturas--us-34--issue-70),
[diferencial Hurl 8.0.0](docs/compatibility.md#renovação-transacional-de-capturas--us-34--issue-70).
As regras US-34 substituem as restrições antigas de ordem e colisão exata.

## Publicar resultados JUnit no CI (US-17 / issue #68)

```bash
httpgo test tests/ --output junit --output-file target/test-results.xml
```

Um arquivo de cenário corresponde a um `testcase`: assertion divergente vira
`failure`, erro de entrada/captura/renderização/transporte vira `error`, e arquivo
não executado por `--fail-fast` vira `skipped`. Os nomes são os caminhos relativos
normalizados da descoberta (fallback absoluto entre unidades); duração em segundos.
O relatório reutiliza a execução, **sem reenviar requisições**.

Exemplo mínimo [versionado](testdata/scenarios/scenario_junit.hurl):

```hurl
GET {{base_url}}/health
HTTP 200
```

```bash
httpgo test testdata/scenarios/scenario_junit.hurl --var base_url=http://localhost:8080 --output junit --output-file target/test-results.xml
```

Requer um servidor com `/health`: 200 retorna **0**, outro status retorna **1** e
produz o artefato com `failure`. `--output junit` exige `--output-file`; cria o
pai quando possível, substitui o destino existente e deixa stdout vazio.
Diagnósticos seguros vão a stderr. Erro de execução ou escrita/fechamento retorna
**2**, que prevalece sobre **1**, sem anunciar publicação bem-sucedida. Falha de
configuração/descoberta anterior à execução não cria nem modifica o relatório.

O XML não inclui bodies/headers integrais e reutiliza a proteção de valores
conhecidos/derivados; segredos arbitrários não marcados não são detectados.
Credenciais vêm do ambiente do processo, nunca de flags. YAML hierárquico e
interpolação externa continuam **extensões HTTPGo**, não recursos do formato Hurl.
Não há envio para serviço externo ou dashboard. HTML offline é o formato opt-in
separado da US-35, sem publicação em CI por este workflow.

O [workflow de referência](.github/workflows/junit.yml) usa
`dorny/test-reporter` **v2.6.0** (`java-junit`), fixado por commit, para validar e
apresentar artefatos gerados pelo binário real. O consumidor apresenta `failure`
e `error` como testes falhos, mantendo a distinção no XML. Detalhes de ingestão,
permissões e reprodução em [docs/junit.md](docs/junit.md).

## Comparar a quantidade antes e depois (US-33 / issue #66)

```hurl
GET {{base_url}}/orders

HTTP 200
[Captures]
orders_before: jsonpath "$.items" count

POST {{base_url}}/orders/preview
Content-Type: application/json
{"productId":"p1"}

HTTP 200

GET {{base_url}}/orders

HTTP 200
[Asserts]
jsonpath "$.items" count == {{orders_before}}
```

Salve como `unchanged.hurl` ou use o
[cenário versionado](testdata/scenarios/scenario_count_unchanged.hurl):

```bash
httpgo test unchanged.hurl --render-asserts --var base_url=http://localhost:8080
```

A API retorna `{"items":[...]}` em `/orders`; `/orders/preview` não deve alterar
sua quantidade. Três itens antes e três depois passam com **0**, mesmo com
conteúdo diferente; quatro depois falham com **1**, sem imprimir os pedidos.

- `nome: jsonpath "consulta" count` publica **número JSON inteiro não negativo**,
  não string. Conta somente elementos do array final selecionado, incluindo
  `null`; array vazio publica zero. O número pode ser reutilizado em URL,
  header, string JSON ou valor JSON completo (mantendo tipo numérico).
- Capturar não exige `--render-asserts`; usar o baseline como esperado dinâmico
  exige esse opt-in HTTPGo. Os comparadores continuam `==`, `>`, `>=`, `<`, `<=`.
  Não há aritmética, `count !=`, contagem de strings/objetos, parâmetros,
  pipelines ou `count` em captura de header.
- Ausência, `null`, escalar/objeto, JSON inválido ou percurso incompatível são
  **erro de captura 2**, nunca baseline zero fictício. Em assertion, ausência ou
  tipo não array continuam divergência **1**. Wildcard/filtro respeitam a
  redução US-27/28: zero resultados é ausência; um resultado conserva seu tipo;
  vários formam array sem achatamento. Um único array conserva seu conteúdo.
- Ordem: status → capturas provisórias → assertions → confirmação atômica →
  apresentação → próximo envio (US-34). Nome exato pode ser renovado; duplicação
  no lote e conflito hierárquico rejeitam tudo. Assertions veem o lote atual.
  Erro estático tardio impede todo HTTP.
- `--secret orders_before` protege a quantidade e seus usos; comparação posterior
  oculta esperado **e** real, mesmo diferentes. Diagnóstico de captura informa
  consulta, `count`, causa e localização, sem despejar a coleção. Falhas de
  transporte/escrita impedem avanço; sessões não compartilham baselines.

[Subconjunto](docs/subset.md#captura-de-contagem--us-33--issue-66),
[diferenças Hurl 8.0.0](docs/compatibility.md#captura-de-contagem--us-33--issue-66)
e [evidência Linux](docs/validation.md#us-33--issue-66--história-0033).
Corpus sintético não equivale à homologação de uma suíte real.

## Verificar mudança de valor JSON (US-32 / issue #64)

```hurl
POST {{base_url}}/session/login
HTTP 200
[Captures]
old_token: jsonpath "$.accessToken"

POST {{base_url}}/session/renew
HTTP 200
[Asserts]
jsonpath "$.accessToken" != "{{old_token}}"
[Captures]
new_token: jsonpath "$.accessToken"

GET {{base_url}}/next
Authorization: Bearer {{new_token}}
HTTP 200
```

Salve como `rotation.hurl` ou use o [cenário versionado](testdata/scenarios/scenario_json_inequality_rotation.hurl):

```bash
httpgo test rotation.hurl --render-asserts --secret old_token --secret new_token --var base_url=http://localhost:8080
```

A API deve retornar `accessToken` string em login/renovação e aceitar o token novo
em `/next`. Novo valor passa (**0**); repetição falha (**1**). Ausência em
`new_token` é erro de captura (**2**, anterior à assertion desde a US-34).
Ambas impedem a próxima requisição/confirmação. JSON inválido, percurso incompatível ou variável
dinâmica ausente são erros (**2**), nunca aprovação por inversão.

- `!=` usa a mesma comparação tipada/exata de `==`: número e string diferem;
  `1`, `1.0`, `1e0` não. Objetos ignoram ordem das chaves; arrays preservam ordem.
  `null` presente é valor e difere de string vazia/número, mas não de `null`.
- Aceita os seis tipos JSON da igualdade e as consultas já disponíveis, sem
  ampliar JSONPath. `HTTP` é obrigatório. `count !=`, header/body `!=` e negação
  genérica continuam rejeitados antes de qualquer envio do arquivo.
- O perfil literal é preservado: sem `--render-asserts`, `"{{old_token}}"` é texto.
  Com a flag (evolução explícita HTTPGo), resolve uma vez sobre o lote provisório,
  confirmados anteriores e iniciais, antes da consulta/decode da assertion alcançada.
- **Marque segredos explicitamente** como acima: `--secret` fornece nomes, nunca
  credenciais. Valores secretos iniciais vêm do ambiente, não de flags. Operandos
  protegidos/derivados e capturas secretas pendentes ficam ocultos em diagnósticos;
  coleções são somente resumos. A ferramenta não detecta todo segredo arbitrário.
- **Diferença não garante revogação do token antigo, validade do novo ou ausência
  de outros efeitos**. Faça requisições/verificações independentes para prová-los.

[Contrato](docs/subset.md#desigualdade-json--us-32--issue-64),
[arquitetura](docs/architecture.md#desigualdade-json--us-32--issue-64) e
[diferenças observadas com Hurl 8.0.0](docs/compatibility.md#desigualdade-json--us-32--issue-64).

## Selecionar com valores anteriores (US-31 / issue #62)

```hurl
GET {{base_url}}/first
HTTP 200
[Captures]
order_number: jsonpath "$.number"

GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"
[Captures]
selected_id: jsonpath "$.items[?(@.number=={{order_number}})].id"

GET {{base_url}}/{{selected_id}}
HTTP 200
```

Execute o [exemplo versionado](testdata/scenarios/dynamic_filter.hurl):
`httpgo test testdata/scenarios/dynamic_filter.hurl --render-asserts --var base_url=http://localhost:8080`.
A fixture requer `/first` retornando `{"number":42}` e `/orders` retornando
`{"items":[{"number":41,"status":"PLACED","id":"wrong"},{"number":42,"status":"PLACED","id":"selected"}]}`.
Apenas o pedido **42** aprova e publica `selected_id` para consultar `/selected`.
Se estiver cancelado ou ausente, retorna **1** sem captura/próximo envio, mesmo
havendo outro pedido `PLACED`. Os testes usam HTTP local; estas não são rotas
prometidas pelo dummy nem homologação de uma API real.

- **Evolução explícita HTTPGo:** a flag US-30 também habilita operandos de filtros
  por igualdade em assertions **e capturas**. Sem ela, placeholders nas consultas
  são rejeitados antes da rede, **inclusive dentro de strings**. Isso substitui
  explicitamente o literal semelhante a placeholder permitido pela US-28, mas
  não altera o modo literal dos **esperados** da US-30.
- `@.number=={{number}}` preserva string/número exato/booleano/`null` da origem;
  `@.name=='Order {{suffix}}'` compõe string com conversões escalares existentes.
  String numérica não vira número. Arrays/objetos, mesmo vazios, retornam **2**.
- Aspas, apóstrofos, barras, colchetes e texto de expressão substituídos são dados
  vinculados ao plano; a expressão inteira nunca é renderizada ou reanalisada.
  Não há expansão recursiva, propriedades/índices/operadores/consulta dinâmicos,
  nem novos operadores. Os dois níveis de escape de literais estáticos permanecem.
- Estrutura inválida em qualquer entrada impede **todo HTTP do arquivo**.
  Valores são resolvidos na consulta alcançada, após preparar seu esperado e
  antes de decode/seleção, mesmo que o array esteja vazio. Variável ausente/tipo
  inválido é avaliação **2**, não divergência ou aprovação por negação.
- Capturas leem somente iniciais e capturas anteriores confirmadas. Assertions
  veem também o lote atual: status → capturas → assertions → confirmação → callback.
  Zero/um/vários e percurso estrito seguem US-27/28, sem escolher primeiro ID.
  Segredos/derivados são protegidos e planos/operandos não vão aos relatórios.

O Hurl interpola consultas sem opt-in e aceita formas estruturais excluídas aqui,
como índice dinâmico. Vinculação tipada, precisão, escapes e publicação podem
divergir: veja [evidência Hurl 8.0.0](docs/compatibility.md#operandos-dinâmicos--us-31--issue-62),
[subconjunto](docs/subset.md#operandos-dinâmicos--us-31--issue-62),
[arquitetura](docs/architecture.md#operandos-dinâmicos--us-31--issue-62) e
[validação](docs/validation.md#us-31--issue-62--história-0031).

## Validar tipos e valores vazios (US-29 / issue #58)

```hurl
GET {{base_url}}/session-info
HTTP 200
[Asserts]
jsonpath "$.token" isString
jsonpath "$.attempts" isInteger
jsonpath "$.active" isBoolean
jsonpath "$.issues" isEmpty
```

Execute o [exemplo versionado](testdata/scenarios/json_types.hurl):
`httpgo test testdata/scenarios/json_types.hurl --var base_url=http://localhost:8080`.
A resposta sintética `{"token":"abc","attempts":3,"active":false,"issues":[]}`
aprova com **0**. `/session-info` é fixture dos testes, não rota prometida pelo
dummy nem homologação de API real. `attempts` como `"3"`, `3.0` ou `3e0` falha
com **1**, sem captura nem próxima requisição.

- Predicados unários, **sem esperado** e sem coerção. `isString` inclui string
  vazia; `isBoolean` inclui false. `isInteger` exige **notação inteira JSON**,
  inclusive negativos e -0, sem limite de máquina nem conversão a float64.
- Igualdade numérica permanece exata: `3`, `3.0` e `3e0` são equivalentes. `count`
  ainda aceita esperado integral decimal/exponencial; isso não é `isInteger`.
- `isEmpty` aprova string/array/objeto vazios. Whitespace, `[null]`, `{"x":null}`,
  null, números e booleanos não passam. Campo/índice ausente falha com **1**,
  distinto de vazio e null. JSON inválido/percurso incompatível mantém **2**.
- Consulta simples, wildcard e filtro estático existentes conservam a redução:
  zero = ausência, um = próprio valor, vários = array; nunca aplicação por item.
- Ordem US-34: status → capturas provisórias → assertions → confirmação. Operando extra, `count`
  combinado com novo unário e construções fora do recorte impedem todo HTTP do
  arquivo. Sem schema, outros tipos, coerção ou negação genérica.
- Diagnósticos categóricos não imprimem valores/estruturas, mesmo escalares;
  metadados protegidos e captura secreta pendente seguem a ocultação existente.
  `isString` e vazio de string/array são atalhos para verificações já possíveis
  com `matches ""`, `== ""` e `count == 0`, não substitutos dos operadores.

[Subconjunto](docs/subset.md#tipos-e-valores-vazios-json--us-29--issue-58),
[arquitetura](docs/architecture.md#tipos-e-vazio--us-29--issue-58),
[compatibilidade Hurl 8.0.0](docs/compatibility.md#tipos-e-vazio--us-29--issue-58)
e [validação](docs/validation.md#us-29--issue-58--história-0029).

## Comparar com valores anteriores (US-30 / issue #60)

`--render-asserts` habilita esperados dinâmicos **somente na invocação atual**,
em todos os arquivos. É evolução explícita do contrato HTTPGo: sem a flag,
o perfil literal preserva placeholders em strings, definidos ou ausentes, e
continua rejeitando placeholder fora de literal válido antes da rede.

```hurl
POST {{base_url}}/orders
HTTP 200
[Captures]
order_number: jsonpath "$.number"
order_name: jsonpath "$.name"

GET {{base_url}}/orders/{{order_number}}
HTTP 200
[Asserts]
jsonpath "$.number" == {{order_number}}
jsonpath "$.name" == "{{order_name}}"
```

Execute o [exemplo versionado](testdata/scenarios/scenario_render_asserts.hurl):
`httpgo test testdata/scenarios/scenario_render_asserts.hurl --render-asserts --var base_url=http://localhost:8080`.
Requer POST e GET retornando `{"number":9007199254740993,"name":"Pizza"}`.
Os E2E usam HTTP local real; não é uma rota prometida pelo dummy nem homologação
da suíte real. Correspondência retorna **0**, outro número retorna **1** sem
perder precisão, capturar a entrada divergente ou enviar a próxima requisição.

- Valor JSON completo preserva tipo, estrutura e precisão onde o operador já os
  aceita; dentro de string, escalar vira texto escapado. `== {{number}}` compara
  número se a origem é numérica; `== "{{number}}"` compara string.
- Comparação ordenada exige número; `count` exige inteiro não negativo exato;
  `contains`/`not contains` JSON aceitam só escalar. Nenhum operador novo.
- Headers/body e regex JSON/header aceitam esperado textual com as conversões
  escalares existentes, nunca array/objeto implícito. Exemplo:
  `header "X-Order" contains "{{order_number}}"` ou
  `jsonpath "$.name" matches "^{{name_pattern}}$"`.
  Metacaracteres inseridos participam da expressão **sem escape automático**.
- Resolução **única**, sem reinterpretar placeholders inseridos. Esperado/regex
  final são preparados antes da consulta da assertion alcançada. Falha de
  variável/tipo/regex é avaliação **2**, inclusive em negação com valor ausente.
  Sintaxe inválida e regex sem referências inválida impedem todo HTTP do arquivo.
- Ordem US-34: status → capturas provisórias → assertions → confirmação atômica.
  Captura própria válida está disponível; futura, não. Segredos/derivados são
  protegidos antes de callbacks; planos e regex operacionais não vão ao resultado.
- A US-31 acima amplia somente operandos de filtros nas consultas. Nomes de
  header, padrões de captura e templates de requisição não mudam. Sem regex de body em assertions, expressões, aritmética ou nova env/YAML.
  As seções legadas abaixo descrevem o perfil literal, salvo indicação explícita.

[Subconjunto](docs/subset.md#esperados-dinâmicos--us-30--issue-60),
[arquitetura](docs/architecture.md#esperados-dinâmicos--us-30--issue-60),
[diferencial Hurl 8.0.0](docs/compatibility.md#perfis-de-esperados--us-30--issue-60)
e [validação](docs/validation.md#us-30--issue-60--história-0030).

## Selecionar a entidade correta pelo conteúdo (US-28 / issue #56)

```hurl
GET {{base_url}}/products
HTTP 200
[Captures]
pizza_id: jsonpath "$[?(@.slug=='pizza')].id"
[Asserts]
jsonpath "$[?(@.slug=='pizza')].active" == true

GET {{base_url}}/products/{{pizza_id}}
HTTP 200
```

Execute o [exemplo versionado](testdata/scenarios/scenario_filter.hurl) com
`httpgo test testdata/scenarios/scenario_filter.hurl --var base_url=http://localhost:8080`.
A API sintética dos testes devolve `[{"slug":"pizza","id":"p1","active":true},
{"slug":"juice","id":"p2","active":false}]`. Mesmo invertendo a ordem, a
consulta seguinte envia `/products/p1`. Pizza inativa falha com **1**, sem captura
nem próxima requisição, mesmo se o outro produto estiver ativo. Não é uma rota
prometida pelo dummy nem homologação de uma API real.

- Filtro estático `[?(@.caminho == literal escalar)]` em assertions/capturas:
  caminho relativo com propriedades por ponto e índices não negativos; literal
  string entre aspas simples/duplas, número JSON exato, booleano ou `null`.
  Há dois níveis de escape: string `.hurl` externa e literal da consulta interna.
- Igualdade tipada sem coerção, inclusive além de `int64`, decimais próximos e
  expoentes extremos. Ausente não corresponde nem a `null`; os passos seguintes
  partem da **mesma entidade**, inclusive com filtros aninhados e wildcards.
- A cardinalidade da US-27 permanece: zero = ausência, um = próprio valor,
  vários = array ordenado, sem achatamento/reordenação/deduplicação. `count` e
  demais predicados recebem esse valor reduzido, não cada correspondência.
- Captura ausente é **2**; `null` único é válido. Coleções mantêm tipos e precisão
  em JSON completo, nunca viram texto em URL/header nem escolhem o primeiro ID.
  Confirmação atômica segue status → capturas provisórias → assertions (US-34).
- Filtro em não array ou percurso incompatível no atributo invalida **toda** a
  seleção com **2**, inclusive em ramos mistos; ausência só elimina o item.
  JSON inválido continua erro de avaliação, não ausência ou aprovação por negação.
- Operadores além de `==`, existência/lógica/regex/funções/scripts em filtro
  continuam fora. Operandos dinâmicos seguem a US-31 acima: placeholders nas
  consultas exigem `--render-asserts`, inclusive em literais.
  Erro estático tardio impede todos os envios; diagnósticos localizados não
  despejam coleção e sempre ocultam literais de filtro/plano operacional.

O adapter OjG navega; **a igualdade de filtros é própria e exata**, não a avaliação
numérica nativa da biblioteca. [Subconjunto](docs/subset.md#seleção-estática-por-igualdade--us-28-issue-56),
[arquitetura](docs/architecture.md#seleção-estática-por-igualdade--us-28--issue-56),
[diferenças Hurl 8.0.0](docs/compatibility.md#filtros-estáticos-por-igualdade--us-28--issue-56)
e [validação](docs/validation.md#us-28--issue-56--história-0028).

## Aguardar condições assíncronas (US-26 / issue #50)

```hurl
GET {{base_url}}/jobs/123
[Options]
retry: 3
retry-interval: 500
HTTP 200
[Asserts]
jsonpath "$.status" == "READY"
```

Execute o [exemplo versionado](testdata/scenarios/scenario_polling.hurl):

```sh
bin/httpgo test testdata/scenarios/scenario_polling.hurl --var base_url=http://localhost:8080
```

O endpoint é sintético dos testes, não uma rota do dummy. Com `PENDING`, `PENDING`,
`READY`, há três envios e saída **0**. Se sempre divergir, há no máximo quatro e
saída **1**, sem próxima entrada. `retry` conta **novas** tentativas (default `0`);
`retry-interval` é inteiro em ms (default `1000`, zero sem espera deliberada).
Negativos, frações, duplicações, overflow e opções fora desse recorte são erros
estáticos **2**, impedindo todo HTTP do arquivo.

- Somente divergência de status/assertion (`1`) repete; erro de transporte,
  avaliação/decodificação, captura, preparação, cancelamento ou escrita (`2`)
  encerra sem retry. Tipo/ausência classificados como assertion falha continuam
  elegíveis, sem converter erros de avaliação em divergência.
- Reenvia a requisição completa, inclusive **POST**, podendo repetir efeitos:
  **não há garantia de idempotência**. Mantém variáveis, headers, campos e caminhos;
  arquivos externos e multipart são relidos/reabertos em cada tentativa. Não há
  snapshot dos bytes diante de alteração externa. Redirects recomeçam no destino
  e método originais em cada tentativa, não na última URL redirecionada.
- Status → capturas provisórias → assertions → confirmação; tentativas divergentes não publicam capturas,
  mas segredos descobertos permanecem protegidos. Respostas/fontes são fechadas
  antes de avaliar/esperar; só aprovação publica o lote de capturas uma vez.
- Espera cancelável só entre tentativas; prazo HTTP por tentativa, sem timeout
  global novo, backoff, espera ilimitada, retry global ou repetição do arquivo.
- Há uma entrada lógica e um resultado terminal com `[tentativas=N]`. Resumos
  separam `tentativas_iniciadas`, `respostas_completas` e `status_comparados` dos
  contadores lógicos anteriores. Falhas transitórias recuperadas não contam como
  falha definitiva nem acionam fail-fast. Cancelamento na espera é **2**.

[Subconjunto](docs/subset.md#polling-local-por-entrada--us-26--issue-50),
[diferenças observadas com Hurl 8.0.0](docs/compatibility.md#polling--us-26--issue-50)
e [validação](docs/validation.md#us-26--issue-50--história-0026).

## Verificar coleções sem índices fixos (US-27 / issue #52)

`[*]` projeta elementos de arrays em assertions e capturas, combinando propriedades,
índices e wildcards aninhados (`$.categories[*].products[*].id`).
Salve como `collections.hurl` e execute
`httpgo test collections.hurl --var base_url=http://localhost:8080`:

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.issues[*].code" contains "REQUIRED"
jsonpath "$.issues[*].code" not contains "OTHER_TENANT"
[Captures]
codes: jsonpath "$.issues[*].code"
POST {{base_url}}/echo
{{codes}}
HTTP 204
```

Requer `/validation` devolvendo `{"issues":[{"code":"REQUIRED"},{"code":"INVALID"}]}`
e `/echo` aceitando o array JSON com status 204. A ordem dos códigos não importa
para pertencimento; retirar REQUIRED ou incluir OTHER_TENANT falha com **1**,
sem captura nem próximo envio. O exemplo versionado é
`testdata/scenarios/scenario_projection.hurl`, executado pelos E2E com HTTP local;
não representa homologação de uma API real.

- A redução acontece **somente ao final**: zero resultados = ausência; um = próprio
  valor; vários = array ordenado por percurso, com repetições. Um número único
  continua número; `count` falha com 1 nesse caso e na ausência, mas conta 2 para
  dois números. `contains` usa substring na string única e pertencimento tipado
  no array reduzido. Nenhum predicado é aplicado implicitamente a cada item.
- Atributo ausente/índice fora do array elimina só o ramo; `null` selecionado é
  presente. Selecionar um array vazio é diferente de expandir um array vazio
  (ausência). Um array selecionado permanece array; vários não são achatados.
- Percurso por tipo incompatível, inclusive `[*]` em objeto/número/null, invalida
  a seleção inteira com **2**, sem aproveitar resultados parciais ou aprovar negação.
- Captura sem resultado falha com **2**; tipos e precisão são preservados no lote
  atômico. Arrays/objetos exigem valor JSON completo, nunca URL/header/string JSON.
- Falhas expansivas mostram consulta/predicado/localização e presença/tipo/contagem,
  sem despejar seleção ou esperado composto. Resumos protegidos ficam ocultos.
  Caminhos simples mantêm seus contratos anteriores.
- Continuam fora: `.*`, slices, unions, descida recursiva e filtros além da igualdade estática da US-28. Validação
  estática integral impede todos os envios do arquivo em caso de consulta inválida.
  Diferenças deliberadas frente ao Hurl 8.0.0 estão em [docs/compatibility.md](docs/compatibility.md#projeção-de-arrays--us-27--issue-52).

## Preservar precisão nas consultas (US-27b / issue #54)

A evolução interna mantém o mesmo subconjunto, sem novas expressões ou opção de
avaliador. Para `{"products":[{"id":9007199254740992},{"id":9007199254740993}]}`:

```hurl
GET {{base_url}}/products

HTTP 200
[Asserts]
jsonpath "$.products[0].id" == 9007199254740992
jsonpath "$.products[1].id" == 9007199254740993
jsonpath "$.products[*].id" count == 2
[Captures]
product_ids: jsonpath "$.products[*].id"

POST {{base_url}}/echo
{{product_ids}}
HTTP 204
```

Execute o [exemplo versionado](testdata/scenarios/scenario_projection_precision.hurl)
com `httpgo test testdata/scenarios/scenario_projection_precision.hurl --var base_url=http://localhost:8080`.
`/products` e `/echo` são contratos sintéticos dos testes locais. A captura envia
`[9007199254740992,9007199254740993]` como JSON completo; trocar o segundo ID pelo
primeiro falha com **1**, sem publicar captura nem enviar o POST.

Números selecionados, comparados e capturados mantêm tipos e precisão além de
`2^53` e de `int64`, decimais próximos e expoentes extremos, sem virar strings,
infinito ou zero por perda de representação. Grafias capturadas são preservadas;
igualdade numérica existente continua aceitando formas equivalentes (`1.00`, `1e0`).
Projeções usam OjG `jp` somente para navegação, caminhos simples permanecem nativos.
A adoção US-27b, por si só, **não** habilita filtros (igualdade estática é a US-28), funções ou conformidade JSONPath/RFC integral nem
altera cardinalidade, erros, sigilo ou restrições de uso textual de coleções.
Veja [arquitetura](docs/architecture.md#projeção-de-arrays--us-27--issue-52) e
[compatibilidade preservada](docs/compatibility.md#preservação-de-jsonpath--us-27b--issue-54).

## Validar inclusão e ausência (US-23 / issue #44)

Salve `validation.hurl` e execute
`bin/httpgo test validation.hurl --var base_url=http://localhost:8080`:

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.codes" contains "REQUIRED"
jsonpath "$.codes" not contains "OTHER_TENANT"
jsonpath "$.passwordHash" not exists
header "Set-Cookie" not contains "Domain"
```

Com `{"codes":["REQUIRED","INVALID"],"name":"Pizza"}` e cookie sem `Domain`,
retorna **0**. Retirar `REQUIRED`, incluir `OTHER_TENANT`, devolver `passwordHash`
(mesmo `null`) ou incluir `Domain` na primeira ocorrência do header retorna **1**,
sem capturas nem próximo envio. Exemplo executável versionado em
[`scenario_assert_membership.hurl`](testdata/scenarios/scenario_assert_membership.hurl).
`/validation` é um contrato sintético dos testes locais, não endpoint do dummy
nem homologação de uma suíte real.

- Strings buscam substring literal, com caixa/whitespace/Unicode preservados;
  somente esperado string é aplicável. Arrays buscam um elemento **direto**, por
  tipo e valor: número não é string numérica. Esperados são escalares JSON;
  números usam igualdade exata, sem float64 ou expansão de expoentes.
- Ausência aprova `not exists` e `not contains`; `null` presente não é ausência.
  Vazios, false e zero também existem. Tipo incompatível falha com **1** nos
  dois sentidos de pertencimento. Array vazio não contém escalar algum;
  toda string contém `""`, logo `not contains ""` falha em string presente.
- Headers ignoram caixa do nome e consultam apenas a primeira ocorrência,
  sem concatenar valores. `not contains` aprova header ausente; presente vazio
  contém `""`. `Set-Cookie` e demais headers sensíveis continuam ocultos.
- Consultas são estruturais estáticas; operandos de filtros seguem US-31.
  Esperados são literais sem `--render-asserts`, inclusive `{{nome}}` em string.
  Não há negação genérica, `header not exists`, `count contains`, busca
  recursiva ou filtros além da igualdade estática US-28; projeção de arrays `[*]` é o recorte US-27 acima. Sintaxe inválida tardia impede todos
  os envios. JSON inválido, percurso incompatível e falha de execução são **2**,
  nunca aprovação por negação. Ordem e publicação de capturas são preservadas.
- Falhas localizadas mostram presença/tipo e esperado escalar protegido, não a
  coleção selecionada. Captura secreta provisória oculta operandos sem confirmá-la;
  segredos arbitrários não marcados não são detectados universalmente.

Os **37 pares `membership_*`** foram confrontados com Hurl **8.0.0**: diferenças
intencionais de headers repetidos (lista versus primeira ocorrência), literais,
expoentes e percurso estão em [compatibilidade](docs/compatibility.md#inclusão-e-ausência--us-23--issue-44).
YAML hierárquico e interpolação de arquivos externos continuam extensões HTTPGo;
esta história não altera esses contratos.

## Detectar conteúdo proibido na resposta (US-21 / issue #40)

Salve `privacy.hurl` e execute
`bin/httpgo test privacy.hurl --var base_url=http://localhost:8080`:

```hurl
GET {{base_url}}/export
HTTP 200
[Asserts]
body contains "customer"
body not contains "passwordHash"
body not contains "trackingToken"
```

A API deve devolver `customer` e nenhum dos textos proibidos. O cenário passa com
**0**; qualquer condição falsa retorna **1** e impede capturas e a próxima
requisição do arquivo. Exemplo versionado:
[`scenario_assert_body.hurl`](testdata/scenarios/scenario_assert_body.hurl).
O dummy não implementa `/export`; os testes usam um servidor HTTP local isolado.

A busca usa o **texto completo recebido da resposta final**, sem reserializar
JSON, distinguir campos ou normalizar caixa, espaços, quebras de linha e Unicode.
O esperado é uma string literal: `{{nome}}` não interpola. Todo texto contém
`""`, inclusive body vazio; `not contains ""` sempre falha. Charset declarado no
primeiro `Content-Type` é respeitado (rótulos web; ISO-8859-1 → windows-1252);
sem charset, usa UTF-8 estrito. Charset desconhecido, bytes inválidos/truncados,
resposta incompleta e erros de consumo/fechamento retornam **2**, nunca aprovação
de ausência. Somente após consumir e fechar a resposta: status → capturas
provisórias → assertions de header/JSON/body na ordem escrita → confirmação. Erro estático tardio impede todo
HTTP do arquivo.

Diagnóstico localizado informa operador/esperado protegido, **nunca body ou
fragmentos recebidos**. Captura secreta provisória oculta esperado antes de sua
confirmação. Não há assertion regex de body nem inspeção de binários.
Interpolação de esperados exige o opt-in US-30 acima. Busca literal não prova ausência de todas as codificações de um dado.
Os exemplos confrontados com Hurl **8.0.0** e as diferenças de códigos estão em
[`docs/compatibility.md`](docs/compatibility.md#ampliação-us-21--busca-textual-no-body-issue-40).

## Executar uma suíte (US-16 / issue #38)

```sh
bin/httpgo test tests/ --env local
bin/httpgo test tests/ --env local --fail-fast
# Também aceita caminhos misturados, repetidos e arquivos sem extensão:
bin/httpgo test tests/ tests/01-health.hurl checks/explicit --env local
```

Com `01-health.hurl` aprovado, `02-products.hurl` divergente e `03-profile.hurl`
aprovado, a primeira chamada executa os três e retorna **1** (dois sucessos e uma
falha de assertion). A segunda não envia o terceiro e o marca **não executado**.
Exemplos em [`testdata/suites/basic/`](testdata/suites/basic/), usando `base_url`:
`bin/httpgo test testdata/suites/basic/ --var base_url=http://localhost:8080`.
A API demonstrativa deve responder 200 em `/health` e `/profile` e 503 em
`/products`; os testes usam servidor isolado, não dependem do dummy.

- Diretórios descobrem `.hurl` recursivamente; arquivos explícitos independem da
  extensão. Normaliza absolutos/relativos, `.` e `..`, deduplica pela identidade
  real do filesystem e respeita sua equivalência de caixa. Ordena pelo caminho
  relativo ao cwd com `/`, independentemente dos argumentos; externos podem ter
  `../`. Sem relativo possível entre unidades, usa absoluto normalizado com `/`.
- Não segue links: descobertos são ignorados, explícitos com link em qualquer
  componente são rejeitados. Descoberta inteira antes de HTTP: caminho inválido,
  inacessível, entrada não regular, falha de acesso ou cada diretório explícito
  sem `.hurl` elegíveis retorna **2**, sem executar nem as entradas válidas.
  Essas verificações não são sandbox contra mudanças concorrentes do filesystem.
- Configuração global é carregada/validada uma vez. Cada arquivo é aberto,
  integralmente pré-validado e fechado antes de iniciar suas requisições. Um erro
  tardio de estrutura impede todos os envios daquele arquivo, não desfaz anteriores.
- Execução sequencial. A primeira falha para as próximas requisições do arquivo;
  sem `--fail-fast`, segue para outros arquivos. Com a flag, os restantes não são
  abertos/analisados para inventar contagens. Cancelamento global ou falha de
  apresentação sempre encerra a suíte, sem reimpressão nem novos envios.
- Cada arquivo começa sem capturas e sem cookies automáticos (não há cookie jar),
  com as mesmas variáveis/opções iniciais. Payloads continuam relativos ao cenário,
  não à configuração. A proteção dos segredos conhecidos/derivados persiste entre
  arquivos, mas não entre invocações; nunca disponibiliza capturas ao próximo.
- Linhas por requisição mantidas; resumo por arquivo acrescenta resultado/duração,
  seguido de `resumo suíte:` com arquivos processados/executados, sucesso, falha de
  assertion, erro, não executados, contadores de requisições e duração de execução.
  Arquivo com erro de pré-validação tem zero envios e `total=desconhecido`;
  `não_iniciadas` também é desconhecido sem parsing. Agregados de requisições
  planejadas cobrem apenas arquivos analisados, com quantidade de arquivos de
  contagem desconhecida explícita. `validadas` conta status aprovado, não sucesso
  completo; ausência de assertion nunca é validação. **2 prevalece sobre 1**.

Sem paralelismo, dependências entre arquivos ou login compartilhado. A US-17
acrescenta JUnit e a US-35 acrescenta HTML offline opt-in; texto continua o padrão.
O contrato detalhado dos contadores
está em [`docs/subset.md`](docs/subset.md#cli-e-saída).

## Executar

Com Go na versão de `go.mod`:

```sh
CGO_ENABLED=0 make build
bin/httpgo --help
bin/httpgo test testdata/scenarios/scenario_get_status.hurl
```

Para usar o servidor dummy do projeto, execute em outro terminal:

```sh
make serve
# Alias equivalente: make dummy
```

Ele escuta somente em `http://127.0.0.1:8080` (também acessível via localhost),
em primeiro plano. Ctrl+C encerra o servidor. A porta precisa estar livre.

| Método | Caminho | Status |
|---|---|---|
| GET | `/health` | 200 |
| GET | `/inexistente` | 404 |
| GET | `/error` | 500 |
| POST | `/products` | 201 |
| GET / PUT / PATCH | `/products/1` | 200 |
| DELETE | `/products/1` | 204 |
| GET / POST / PUT / PATCH / DELETE | `/echo` | 200 |

Os produtos são demonstrativos, **sem persistência**. `/echo` devolve os bytes do
JSON e os headers de resposta `X-Dummy-Method` e `X-Dummy-Body-Bytes`, sem ecoar
headers de autenticação/cookies. Use apenas dados fictícios.

O cenário `scenario_get_status.hurl` executa `/health`, `/inexistente` e repete
`/health`, validando três respostas em sequência. Também é possível executar:

```sh
bin/httpgo test testdata/scenarios/scenario_unvalidated_status.hurl
# POST de demonstração e sequência de métodos da US-02:
bin/httpgo test testdata/scenarios/scenario_json_inline.hurl
bin/httpgo test testdata/scenarios/scenario_dummy_methods.hurl
```

Esse cenário recebe 500 e informa explicitamente que não validou status.
O dummy em `tools/dummy/main.go` é um utilitário **exclusivamente manual** de
desenvolvimento, sem dependências novas. Não integra o binário httpgo nem os
pacotes do build, lint ou testes automáticos. Os testes continuam usando seus
próprios servidores isolados em portas dinâmicas: **não precisam de `make serve`**.
Veja [servidor dummy](docs/dev-server.md) para os detalhes.

```hurl
GET http://localhost:8080/health
HTTP 200

GET http://localhost:8080/inexistente
HTTP 404
```

Também são suportados `POST`, `PUT`, `PATCH` e `DELETE`, com headers e um valor
JSON inline opcional (objeto, array ou escalar; inclusive multilinha). GET aceita
headers, mas permanece sem body. Exemplo contra uma API local com `/products`:

```hurl
POST http://localhost:8080/products
Content-Type: application/json
{"name":"Pizza","price":4500}

HTTP 201
```

Salve como `products.hurl` e execute `bin/httpgo test products.hurl`. O mesmo
exemplo está em `testdata/scenarios/scenario_json_inline.hurl` e é exercitado
pelo binário nos E2E. O dummy manual também implementa essa demonstração: basta
iniciar `make serve`. Headers e JSON são validados antes de qualquer envio, e os bytes do valor
JSON estático são preservados, sem serialização. Declare Content-Type quando necessário;
não há inferência automática desse header.

## Reutilizar configuração do projeto — extensão HTTPGo (US-11 / issue #27)

No diretório corrente, salve `.httpgo.yaml`:

```yaml
version: 1
defaults:
  timeout: 2.5
variables:
  api:
    baseUrl: http://localhost:8080
  product:
    price: 4500
secrets: [auth.token, password]
```

Em `products.hurl`:

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"price": {{product.price}}}
HTTP 201
```

```sh
bin/httpgo test products.hurl
# Somente o arquivo indicado, com override não sigiloso do destino:
bin/httpgo test products.hurl --config environments/local.yaml \
  --var api.baseUrl=http://localhost:9090
# Exemplo versionado, também utilizável com o dummy manual:
bin/httpgo test testdata/scenarios/scenario_config_products.hurl \
  --config testdata/config/project.yaml
```

A API recebe `price` numérico e o prazo total por requisição é 2,5 segundos.
Sem flag, descobre **somente** `.httpgo.yaml` do cwd, nunca dos ancestrais.
Ausência do opcional ou arquivo vazio/só comentários mantém defaults internos;
arquivo explícito inexistente ou selecionado ilegível é erro **2**, sem HTTP.
O diretório da configuração não muda a origem de payloads: continuam relativos
à localização do `.hurl`.

Precedência: **defaults internos < YAML base < ambiente selecionado < CLI**. Mapas de `variables`
definem caminhos exatos, como `api.baseUrl`, não um objeto interpolável `api`.
Strings (inclusive `"001"`), números exatos, booleanos, null e arrays JSON-compatíveis
preservam tipos; objetos dentro de arrays são conteúdo, não novos caminhos.
`--var` substitui o mesmo nome ou acrescenta outro, preservando irmãos. Listas,
null e escalares substituem integralmente; objetos CLI continuam opacos.
Duplicações na mesma fonte e conflitos ancestral/descendente continuam inválidos.

`defaults.timeout` aceita **número de segundos entre 0.001 e 86400**, inclusive,
com precisão de milissegundos inteiros; não aceita `2s` nem arredonda. Omitido,
vale **10 segundos**. Um orçamento cobre conexão/TLS, redirects e consumo da
resposta, respeitando cancelamento/prazo menor do chamador; cada requisição nova
recebe um novo orçamento. Não interrompe I/O local bloqueado pelo SO.

Limite YAML: **1 MiB (1.048.576 bytes)**, um documento, raiz mapa, `version: 1`
obrigatória; opcionais somente `defaults.timeout`, `variables` e `secrets`.
Rejeita campos/chaves/tipos/nomes inválidos, duplicações, âncoras, aliases e tags
explícitas, inclusive aninhados. Referências `${NOME}` seguem a extensão US-13
abaixo: somente valores string completos de folhas de `variables`, fora de listas.
Erros controlados incluem arquivo/posição quando identificáveis, sem trecho bruto.

`secrets` une marcações com `--secret`; overrides não removem proteção. Pode
marcar capturas futuras sem criar valores. Valores conhecidos base/substitutos,
capturados e derivados permanecem protegidos em stdout/stderr/diagnósticos,
inclusive antes da sessão. Não há detecção universal de segredos arbitrários.
**Credenciais reais em flags continuam proibidas**: a proteção não cobre histórico
ou listagem de processos. YAML em disco não é um cofre; proteja arquivos com
permissões `0600`. Overrides automáticos `HTTPGO_*`, stdin e hooks
continuam fora do suporte. Referências explícitas de processo: seção US-13 abaixo.
YAML é **extensão HTTPGo**, não parte do formato Hurl. Contrato completo: [subconjunto](docs/subset.md).

## Selecionar ambientes — extensão HTTPGo (US-12 / issue #29)

Na base `.httpgo.yaml`:

```yaml
version: 1
variables:
  product:
    active: true
```

Em `environments/local.yaml`:

```yaml
variables:
  api:
    baseUrl: http://localhost:8080
  product:
    name: Pizza
    price: 4500
```

Crie `environments/staging.yaml` com os mesmos caminhos, mudando destino para
`http://localhost:9090`, nome para `Suco` e preço para `1200`. Em `tests/products.hurl`:

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}
HTTP 201
```

```sh
bin/httpgo test tests/products.hurl --env local
bin/httpgo test tests/products.hurl --env staging
# Exemplos versionados (local também funciona contra o dummy manual):
bin/httpgo test testdata/scenarios/scenario_environment_products.hurl \
  --config testdata/config/environment_base.yaml --env local
```

Sem editar cenário/payload, cada servidor recebe seus dados, com preço numérico
e `active` booleano herdado; status 201 retorna **0**. `--var product.price=99`
prevalece sobre o ambiente sem apagar irmãos. Listas e `null` substituem folhas
inteiras, nunca concatenam ou removem valores; mapas combinam profundamente por
caminho. Trocar mapa de caminhos por valor ou vice-versa é erro, mesmo mapa vazio.
Objetos CLI continuam opacos; conflitos em qualquer fonte não podem ser escondidos
por overrides posteriores.

`--env <nome>`/`--env=nome` seleciona **um único** ambiente. Repetir a opção, mesmo
com nome igual, é erro. Nome não vazio aceita somente letras ASCII, números, `_`
e `-`, sem ponto, espaço, separador ou traversal. Carrega **somente**
`environments/<nome>.yaml`, relativo ao diretório da base selecionada por
`--config`/descoberta, **mesmo vazia**. Sem base, usa cwd, nunca o diretório do
cenário ou ancestrais. Não muda a origem dos payloads. Sem `--env`, nenhum ambiente
é aberto. Arquivo solicitado ausente/ilegível é erro **2**, sem fallback nem HTTP.

O documento de ambiente admite `defaults`, `variables` e `secrets`, com
`version: 1` **opcional**. Vazio/só comentários não altera a base ou CLI. Valem os
mesmos tipos, limite de 1 MiB por arquivo, precisão/faixa de timeout e proteções
YAML da base. Cada fonte é validada antes da combinação. Marcações de segredos
acumulam base + ambiente + CLI: listas vazias e substituições não desprotegem
valores conhecidos anteriores, novos valores ou capturas futuras. Contexto,
marcas, timeout e capturas não vazam entre execuções.

**Ambiente nomeado não é ambiente do processo.** Referências explícitas `${NOME}`
são resolvidas conforme a US-13 abaixo. Sem composição de vários ambientes,
seleção automática, fontes `HTTPGO_*` ou stdin. Configuração hierárquica
e seleção nomeada são extensões HTTPGo, não sintaxe Hurl.

## Injetar credenciais pelo ambiente do processo — extensão HTTPGo (US-13 / issue #32)

Em `environments/local.yaml`:

```yaml
variables:
  auth:
    token: ${OWNER_TOKEN}
```

Em `tests/profile.hurl`:

```hurl
GET http://localhost:8080/profile
Authorization: Bearer {{auth.token}}
HTTP 200
```

Forneça `OWNER_TOKEN` externamente ao processo (por exemplo, pela configuração
segura do job de CI) e execute, **sem credencial nos argumentos**:

```sh
bin/httpgo test tests/profile.hurl --env local
# Exemplo versionado; requer servidor com /profile:
bin/httpgo test testdata/scenarios/scenario_process_profile.hurl \
  --config testdata/config/process_base.yaml --env process
```

Também funciona na configuração base, em `variables`, com `version: 1`, sem
`--env`. O servidor recebe o token no header; status 200 retorna **0**.
Divergência de assertion retorna **1**, com esperado/recebido sanitizados, inclusive
se a resposta ecoar o segredo em header comum ou JSON.

- `${NOME}` ocupa o **valor string completo**, com nome `[A-Za-z_][A-Za-z0-9_]*`.
  Mapas continuam definindo caminhos como `auth.token`; referências em listas,
  inclusive objetos/listas internos, não são suportadas.
- `Bearer ${OWNER_TOKEN}`, várias referências no mesmo valor, nomes inválidos e
  delimitadores malformados retornam **2**, mesmo com override posterior.
  Cada fonte é validada antes do merge.
- Aplica **base < ambiente < CLI**, depois consulta **somente referências efetivas**.
  Referência substituída não exige a variável do processo. Todas as efetivas são
  obrigatórias, mesmo não utilizadas no cenário. Ausência retorna **2 antes de HTTP**,
  com nome da referência, chave de configuração e origem/posição identificável,
  sem valores nem dump do ambiente.
- Resultado é sempre **string**: `123`, `true`, `null` e JSON recebido não viram
  números, booleanos, null ou objetos. Presente mas vazio é string vazia, diferente
  de ausência e sujeita às validações do contexto de uso. Conteúdo `${OUTRA}` ou
  `{{outra}}` é literal, sem segunda consulta/expansão.
- O destino ganha **proteção automática antes dos overrides**. A marca persiste
  em substituições e se une a `secrets`/`--secret`. `--var auth.token=ficticio`
  pode fornecer substituto não sigiloso, mantendo precedência e proteção;
  **credenciais reais em argumentos continuam proibidas**, inclusive `--var`.
- Valores conhecidos, substitutos e derivados são protegidos antes da primeira
  sessão e de diagnósticos, assertions e callbacks, preservando dados operacionais.
  A proteção cobre variantes conhecidas, não segredos desconhecidos ou transformações
  arbitrárias; não protege o ambiente contra acesso por outros processos privilegiados.

Sem importar outras variáveis do sistema, overrides automáticos `HTTPGO_*`, dotenv,
stdin, composição parcial ou referências em listas. É **extensão HTTPGo**,
não sintaxe Hurl; YAML continua fora do corpus diferencial.

## Payload de arquivo externo (US-09 / issue #22)

```hurl
POST http://localhost:8080/products
Content-Type: application/json
file,../payloads/product.json;
HTTP 201
```

Salve em `tests/products.hurl`, crie `payloads/product.json` e execute
`bin/httpgo test tests/products.hurl`. Exemplo pronto contra o dummy:
`bin/httpgo test testdata/scenarios/scenario_external_body.hurl`.

`file,<caminho>;` substitui o body inline inteiro, não pode ser combinado com
outro body e não é aceito em GET. O caminho é literal, incluindo espaços entre
`file,` e o `;` final; não há sintaxe especial de aspas/escapes nem interpolação.
Nomes com vírgula/ponto e vírgula ficam fora deste recorte. Caminhos relativos
partem do diretório do `.hurl`, independentemente do diretório corrente;
absolutos são aceitos. **Sem sandbox:** `..` e caminhos absolutos podem alcançar
arquivos fora da raiz do projeto, com as permissões do usuário da CLI.

Sem `--render-files`, todos os bytes são enviados sem normalização, validação JSON ou renderização:
vazio, não JSON, binário e `{{placeholder}}` literal, definido ou não. Content-Type
não é inferido. O arquivo é lido integralmente e fechado somente na vez da
requisição, antes do envio. Falha de abertura/leitura/fechamento retorna 2 com
posição da referência e causa segura; aquela requisição e as seguintes não são
enviadas. As anteriores permanecem executadas, sem rollback. O prazo configurado (default de 10s) é do
transporte e não promete interromper leitura local bloqueada pelo sistema.
Não há limite de tamanho: a memória acompanha o arquivo corrente.
Não se imprimem conteúdo nem caminho operacional nos diagnósticos. A proteção
não descobre segredos arbitrários no payload; use somente cenários confiáveis.

**Mudança de contrato:** a antiga rejeição global de `file,...;` foi removida;
negativos específicos mantêm as restrições. Esse modo bytes continua sendo o
padrão. Veja [subconjunto](docs/subset.md) e [evidência diferencial](docs/compatibility.md).

## Parametrizar JSON externo — extensão HTTPGo (US-10 / issue #25)

Em `payloads/product.json`:

```json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}
```

Use o mesmo `tests/products.hurl` acima e execute:

```sh
bin/httpgo test tests/products.hurl --render-files \
  --var product.name=Pizza --var product.price=45 --var product.active=true
bin/httpgo test --render-files tests/products.hurl \
  --var product.name=Suco --var product.price=12 --var product.active=false
```

A API recebe JSONs distintos, com string, número e booleano preservados, sem editar
arquivos. Exemplo versionado: `testdata/scenarios/scenario_external_render.hurl`
e `testdata/payloads/product_template.json`; funciona contra `/products` do dummy.

`--render-files` vale somente para a execução atual e trata **todos** os arquivos
usados como body completo como templates JSON, independentemente de extensão ou
Content-Type. Não infere headers e **não renderiza arquivos multipart**. Caminhos
continuam literais; não se interpolam nomes de arquivos nem chaves JSON.
O mesmo renderer do inline aceita valores completos de qualquer tipo, inclusive
na raiz (null, arrays e objetos); em strings só escalares, escapados corretamente.
Nomes pontuados resolvem exatamente; conteúdo inserido não é reinterpretado.
Variáveis iniciais e capturas anteriores são as únicas fontes existentes.
JSON válido sem placeholders conserva todos os bytes, inclusive whitespace externo.

A leitura, validação e renderização do payload ocorrem **somente na vez da sua
requisição**. Vazio, whitespace sem valor, não JSON, UTF-8 inválido, dois valores,
template malformado, variável ausente, tipo incompatível ou JSON final inválido
retornam 2, sem enviar a entrada afetada nem as seguintes. Anteriores não são
desfeitas. Erros de conteúdo são de renderização, identificam o payload com
linha/coluna originais em bytes quando possível (senão 1:1) e a referência no
cenário. Erros de I/O continuam sendo de arquivo e apontam a referência `.hurl`.
A estrutura inválida do próprio cenário ainda impede **todos** os envios.
Nenhum payload completo é impresso; segredos marcados e derivados/escapes conhecidos
continuam protegidos, sem detecção universal de segredos arbitrários.

Esta é uma **extensão HTTPGo**, separada do subconjunto Hurl: não se promete que
Hurl renderize esses arquivos, nem se fornece a flag própria ao diferencial.

## Upload multipart (US-20 / issue #24)

```hurl
POST {{base_url}}/documents
Authorization: Bearer {{token}}
[MultipartFormData]
description: Documento de teste
category: {{category}}
document: file,../fixtures/document.pdf; application/pdf
HTTP 201
```

Salve em `tests/documents.hurl`, crie `fixtures/document.pdf` e execute contra
uma API que aceite o formulário e responda 201:

```sh
bin/httpgo test tests/documents.hurl --var base_url=http://localhost:8080 \
  --var category=reports --var token=ficticio --secret token
```

O token acima é **fictício**; credenciais reais nunca devem vir em argumentos.
Exemplo versionado: `testdata/scenarios/scenario_multipart.hurl`, com payload
adjacente em `testdata/payloads/`. Não requer frontend nem o dummy manual.

POST/PUT/PATCH aceitam exatamente **um arquivo e zero ou mais campos textuais**.
Nomes são estáticos/únicos; outro body na mesma requisição é erro. Só valores
textuais interpolam, sem conversão de arrays/objetos nem interpretação JSON.
Bytes do arquivo nunca são renderizados, inclusive placeholders indefinidos,
mesmo com `--render-files` ativo.
Caminho literal segue a US-09: relativo ao `.hurl` ou absoluto, sem sandbox.
Filename contém só o basename, preservando espaços/Unicode, sem caminho local.
MIME explícito prevalece; se omitido, usa tabela determinística por extensão
([contrato completo](docs/subset.md)), com fallback `application/octet-stream`.

Boundary e Content-Type são gerados automaticamente. O cenário pode omitir o
header ou declarar `Content-Type: multipart/form-data` sem boundary manual;
outro tipo ou qualquer boundary manual é erro. Parâmetros compatíveis, como
`charset=utf-8`, são preservados. Arquivo é enviado incrementalmente, sem buffer
proporcional ao seu tamanho. O descritor fica aberto durante o executor e fecha
antes de assertions/capturas/resultados, também em falha. Há prazo único configurável (default de 10s)
por requisição/redirects/resposta e cancelamento; I/O local bloqueado pelo SO
continua não sendo interrompível por essa garantia.

Estrutura/metadados estáticos inválidos impedem **todo** envio do arquivo.
Variável ausente, texto composto inválido, Content-Type renderizado proibido ou
falha local retornam 2 antes da entrada afetada, preservando anteriores.
413 declarado/recebido passa (0); status divergente interrompe com 1. Conteúdo e
campos brutos não são impressos; segredos conhecidos e derivados são sanitizados,
mas não se descobrem segredos arbitrários dentro do arquivo.

**Mudança de contrato:** `[MultipartFormData]` deixa de ser globalmente rejeitado;
negativos específicos preservam GET/DELETE sem multipart, exclusividade e limites.
Sem múltiplos arquivos, repetição de campos, filename customizado, headers por
parte, partes JSON tipadas, multipart aninhado ou renderização do arquivo multipart.
O [diferencial Hurl 8.0.0](docs/compatibility.md) registra diferenças reais de
gramática, filename e restrições; não há promessa de compatibilidade completa.

## Parametrizar URL e headers (US-03 / issue #11)

Salve o exemplo abaixo como `products.hurl`:

```hurl
GET {{api.baseUrl}}/products
Authorization: Bearer {{token}}

HTTP 200
```

```sh
bin/httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var token=valor-ficticio --secret token
# Troque somente api.baseUrl para executar o mesmo arquivo em outro destino.
```

O cenário também está em `testdata/scenarios/scenario_products_variables.hurl`;
requer uma API com `GET /products` retornando 200 (o dummy manual não oferece esse
GET). Os E2E demonstram dois destinos locais isolados sem editar o cenário.

`--var nome=valor` é repetível. **Mudança de contrato na US-04:** um literal JSON
válido preserva seu tipo; o restante permanece texto, inclusive vazio. Strings JSON
perdem as aspas delimitadoras e interpretam escapes. O primeiro `=` separa nome/valor;
os seguintes são preservados. Nomes ASCII pontuados, como `api.baseUrl`, resolvem por
nome exato. Duplicações e conflitos ancestral/descendente, como `api`/`api.baseUrl`,
são erros independentemente do tipo. Duplicações são rejeitadas na mesma fonte;
entre YAML e CLI, somente o mesmo nome admite override explícito. Objetos são opacos:
`--var 'product={"name":"Pizza"}'` **não** define `product.name`.
URL e valores de headers aceitam `{{nome}}` e `{{ nome }}`, sem recursão. Nesses
contextos só são aceitos escalares; arrays/objetos geram erro antes do envio.
A URL final exige encoding explícito; nomes de headers permanecem estáticos.

## Parametrizar JSON preservando tipos (US-04 / issue #13)

Salve como `products.hurl`:

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}

HTTP 201
```

```sh
bin/httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var product.name=Pizza --var product.price=4500 \
  --var product.active=true --var 'product.code="001"'
```

A API recebe `name` e `code` como strings, `price` como número e `active` como
booleano. O cenário versionado é `testdata/scenarios/scenario_products_typed.hurl`;
os E2E verificam os dados recebidos por HTTP local. O dummy pode ser usado
manualmente, sem alterar seu código ou depender dele nos testes.

Placeholder sem aspas em posição de valor JSON completo preserva qualquer tipo,
inclusive `null`, array e objeto, também na raiz (`{{body}}`). Dentro de strings
JSON, aceita apenas escalares e composição textual, com escape correto de aspas,
barras invertidas e controles, preservando Unicode. Números conservam sua
representação JSON sem arredondamento, inclusive dentro de estruturas. Conteúdo
inserido semelhante a placeholder permanece literal. Chaves JSON são estáticas;
expressões e acesso a propriedades não são suportados. JSON externo exige
`--render-files`, conforme a extensão US-10 acima.

A interpretação tipada de `--var` é uma **extensão própria HTTPGo**, não promessa
de equivalência com a CLI do Hurl. A sintaxe contextual compartilhada e as diferenças
observadas contra Hurl 8.0.0 estão em [compatibilidade](docs/compatibility.md).

**É proibido fornecer credenciais reais em argumentos, inclusive `--var`.** Use
somente valores não sigilosos ou fictícios. `--secret nome` apenas marca proteção
nas saídas; não fornece valor nem protege histórico de shell/listagem de processos.
Marcar um nome ainda ausente é permitido, mas não resolve sua referência. A fonte
para credenciais iniciais pelo ambiente do processo está disponível por referências
explícitas `${NOME}` em YAML (extensão US-13 acima); stdin não está implementado.

### Capturas literais de headers — US-22 / issue #42

Salve `cache.hurl` (também em
[`testdata/scenarios/scenario_capture_cache.hurl`](testdata/scenarios/scenario_capture_cache.hurl)):

```hurl
GET {{base_url}}/catalog
HTTP 200
[Captures]
catalog_etag: header "ETag"

GET {{base_url}}/catalog
If-None-Match: {{catalog_etag}}
HTTP 304
```

Execute `httpgo test cache.hurl --var base_url=http://localhost:8080` contra uma
API que ofereça cache condicional em `/catalog` (o dummy manual não promete esse
endpoint). O segundo envio recebe o ETag real, incluindo aspas.

O nome é literal e consultado sem distinguir caixa. Somente a **primeira
ocorrência**, inclusive vazia, é capturada; não junta valores nem divide vírgulas.
O valor permanece string, mesmo se parecer número, booleano, JSON ou placeholder,
sem interpolação recursiva. Body não JSON não impede uma captura só de header.
Ausência gera erro de captura **2**; vazio publica string vazia. Só a resposta
final completa participa, após status aceito; lotes mistos com JSONPath são
confirmados atomicamente após assertions, com renovação exata e isolamento por execução.

`Authorization`, `Proxy-Authorization`, `Cookie` e `Set-Cookie` protegem também o
nome capturado e seus usos compostos/escapes conhecidos, sem alterar o valor
enviado. Marcações explícitas continuam acumulando. Não há detecção universal de
segredos ou codificações arbitrárias. Capturar `Set-Cookie` **não** ativa cookie
jar, não extrai atributos nem cria `Cookie`; extração de cookie é escopo da US-25.
O único filtro de captura de header é `regex`, na US-25 abaixo; demais filtros e seletores continuam fora do recorte.

Há diferenças deliberadas frente ao Hurl 8.0.0 em repetição, interpolação JSON,
ordem de assertions e colisões: veja [a matriz diferencial](docs/compatibility.md#capturas-literais-de-headers--us-22--issue-42).
A proteção HTTPGo não é anúncio de equivalência de segurança com Hurl.

Capturas de respostas estão disponíveis desde a US-05, ampliadas para qualquer
tipo JSON na US-06.

## Extração de OTP e token por regex — US-25 / issue #48

```hurl
GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
otp_code: regex "verificação é: ([0-9]{6})"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{otp_code}}"}
HTTP 200
```

Execute `httpgo test otp.hurl --secret otp_code` com as URLs não secretas
configuradas; `--secret` marca a futura captura, não fornece seu valor.
Para renovação explícita de sessão:

```hurl
GET {{base_url}}/session
HTTP 200
[Captures]
refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"

GET {{base_url}}/next
Cookie: refresh={{refresh_token}}
HTTP 200
```

Extrai **somente o primeiro grupo da primeira correspondência**, como string.
Grupo participante vazio publica string vazia; sem correspondência, sem grupo ou
primeiro grupo não participante gera erro de captura **2**, sem próximo envio.
Padrão é string JSON estática, usando regex Go da US-15, sem interpolação.
Erro estático tardio rejeita todo o arquivo antes da rede. Body usa decodificação
textual estrita da US-21; header usa a primeira ocorrência, sem decodificar body.
Status antecede capturas; assertions usam o lote provisório e somente aprovação
confirma atomicamente, permitindo substituição exata (US-34). Grupos de headers sensíveis herdam proteção imediatamente, inclusive
se outra captura falhar. OTP/dados do body exigem marcação explícita; não há
detecção automática de segredos arbitrários. Não há cookie jar, escolha de grupo,
substituições, pipelines ou regex dinâmica. Assertions podem usar o lote atual (US-34).

Cenários executáveis: [`scenario_capture_otp.hurl`](testdata/scenarios/scenario_capture_otp.hurl)
e [`scenario_capture_refresh.hurl`](testdata/scenarios/scenario_capture_refresh.hurl).
Veja [o recorte](docs/subset.md#extração-regex-de-body-e-header--us-25-issue-48)
e [a matriz diferencial](docs/compatibility.md#extração-regex--us-25--issue-48),
que distingue grupos, strings JSON, charset, headers repetidos e códigos de saída.

## Autenticar e reutilizar token (US-05 / issue #14)

Salve como `products.hurl` (também em
`testdata/scenarios/scenario_auth_capture.hurl`):

```hurl
POST {{base_url}}/auth/login
Content-Type: application/json
{"username":"{{username}}","password":"{{password}}"}

HTTP 200
[Captures]
accessToken: jsonpath "$.accessToken"

GET {{base_url}}/products
Authorization: Bearer {{accessToken}}
HTTP 200
```

```sh
bin/httpgo test products.hurl --var base_url=http://localhost:8080 \
  --var username=demo --var password=senha-ficticia \
  --secret password --secret accessToken
```

Requer API que devolva `{"accessToken":"token-ficticio"}` no login e aceite esse
Bearer em `/products`; o dummy manual não implementa autenticação. Os E2E usam
servidor HTTP isolado e verificam o token efetivamente recebido. Todas as
credenciais do exemplo são **fictícias**; credenciais reais em argumentos continuam
proibidas.

`[Captures]` associa-se à resposta anterior, depois de `HTTP` quando declarado.
JSONPath aceita raiz `$`, propriedades por ponto (`$.product.id`), índices de
array não negativos (`$.items[0]`), `[*]` de arrays (US-27) e igualdade estática (US-28). Desde a US-06, o resultado aceita
qualquer tipo JSON, inclusive raiz, `null`, string vazia, arrays/objetos vazios.
JSON inválido, caminho ausente ou índice fora do array é erro de captura (código 2).
Consultas não suportadas em qualquer entrada impedem todos os envios.

Capturas ficam provisoriamente disponíveis às assertions da própria resposta
após consumir/fechar, aprovar o status e extrair/validar todo o lote. Só após
aprovação das assertions e contexto ativo ficam confirmadas para URLs, valores de
headers, JSON inline e JSON externo com `--render-files` seguintes. Strings
como `"123"`, `"null"` ou `"{{literal}}"` continuam strings, sem reinterpretação.
Nomes pontuados resolvem por nome exato, que pode substituir inicial ou captura
anterior. Duplicações no lote e conflitos hierárquicos falham sem confirmação parcial. Cada arquivo
começa sem capturas, inclusive quando vários arquivos compõem uma suíte. `--secret` pode
marcar uma captura futura sem criá-la; valores descobertos ficam ocultos nos
streams e diagnósticos, inclusive variantes de escape conhecidas. Não há garantia
de reconhecer segredos arbitrários antes de descobri-los ou codificações desconhecidas.

## Encadear operações com capturas tipadas (US-06 / issue #16)

Salve como `products.hurl` (também em
`testdata/scenarios/scenario_capture_products.hurl`):

```hurl
POST {{base_url}}/products
Content-Type: application/json
{"name":"Pizza"}

HTTP 201
[Captures]
productId: jsonpath "$.id"

GET {{base_url}}/products/{{productId}}
HTTP 200

POST {{base_url}}/echo
Content-Type: application/json
{"productId":{{productId}}}
HTTP 200
```

```sh
bin/httpgo test products.hurl --var base_url=http://localhost:8080
```

Uma API que retorna `{"id":9007199254740993}` recebe o caminho
`/products/9007199254740993` e o último body com esse mesmo ID **numérico**, sem
arredondamento. Os três status são validados. Os E2E verificam esses bytes por
HTTP real e também o fluxo login → token → criação → ID → consulta → echo de
`testdata/scenarios/scenario_capture_auth_products.hurl`. O dummy manual não
implementa esse contrato dinâmico de ID/autenticação.

**Ampliação intencional do contrato US-05:** capturas agora preservam string,
número, booleano, `null`, array e objeto em posição de valor JSON completo,
inclusive na raiz. Números mantêm precisão também dentro de estruturas; strings
semelhantes a JSON ou placeholders não são reinterpretadas. Em URL, header ou
string JSON, escalares usam sua representação textual (string decodificada,
número JSON, `true`/`false`, `null`); arrays/objetos falham antes do envio dependente.
Não há percent-encoding automático; escape JSON e restrições de headers continuam.
Capturar `product` como objeto não cria `product.id`: declare outra captura com
nome não conflitante, por exemplo `productId: jsonpath "$.id"`. Colisões seguem
proibidas em qualquer tipo; publicação do lote é atômica. `--secret` acompanha
capturas escalares e compostas e protege também seus constituintes e escapes
conhecidos sem mudar os dados enviados nem criar variáveis para as propriedades.

A estrutura do arquivo inteiro é validada antes da rede. Variáveis ausentes e
valores finais inválidos falham imediatamente antes da requisição dependente,
sem contá-la como iniciada e sem enviar as seguintes. As requisições são
sequenciais e a primeira falha de status, header, JSON, body, captura, renderização ou transporte
interrompe as próximas requisições do mesmo arquivo; outros arquivos seguem
conforme `--fail-fast`. Uma falha de captura preserva a resposta concluída e o
status aprovado nas contagens, mas impede o próximo envio. Sem `HTTP`, qualquer status recebido é aceito,
com indicação explícita de **sem validação de status**.

- `0`: sucesso; `1`: status ou assertion de header/JSON/body divergente; `2`: demais erros,
  inclusive avaliação de JSON inválido ou percurso incompatível.
- Timeout total configurável em YAML, default de 10 segundos por requisição,
  incluindo redirects e leitura do body.
- TLS verificado; até 10 redirects, apenas na mesma origem; nenhum retry implícito.
  Polling opt-in por entrada segue a seção US-26 acima.
- Cada resultado é emitido em stdout assim que a requisição termina, antes da próxima;
  resumo por arquivo e agregado ao final, diagnósticos em stderr. Sempre texto sem cores.
- 301/302/303 convertem POST/PUT/PATCH/DELETE em GET sem body; 307/308 preservam ambos.
- Bodies e coleções de headers não são impressos. Apenas valores comuns sanitizados
  podem aparecer no diagnóstico de assertion; valores sensíveis nunca são expostos.
- Credenciais, query inteira e fragmento da URL não são exibidos.
  Valores marcados com `--secret` também são ocultos em paths e headers comuns,
  antes de escapes/normalização de URL. O restante do path permanece visível:
  não coloque segredos arbitrários nele. A CLI não detecta toda credencial ou
  codificação desconhecida automaticamente. A proteção cobre formas brutas,
  decodificadas e escapes conhecidos (JSON, quoting de diagnóstico e URL), inclusive
  em falhas anteriores à construção do contexto. Não há payload de diagnóstico.

## Validar headers da resposta (US-07 / issue #18)

Salve como `health.hurl` e execute `bin/httpgo test health.hurl` contra uma API
que devolva `Content-Type: application/json; charset=utf-8` e `X-Request-Id: req-123`:

```hurl
GET http://localhost:8080/health
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
header "X-Request-Id" exists
header "X-Request-Id" == "req-123"
```

O cenário também está em `testdata/scenarios/scenario_assert_headers.hurl` e é
verificado por HTTP real nos E2E. O dummy manual não promete esses headers.
Esta é uma **ampliação intencional**: `[Asserts]`, antes rejeitada globalmente,
passou a aceitar essas três formas, com strings literais entre aspas duplas.
A US-08 acrescenta igualdade JSON na mesma lista ordenada (abaixo).
Nomes são case-insensitive; igualdade e substring distinguem caixa dos valores.
`{{nome}}` no esperado permanece literal, sem interpolação. A US-23 acrescenta
`not contains` (acima); a US-24 acrescenta `matches` (abaixo). Filtros e demais
operadores continuam não suportados. Capturas de headers são suportadas separadamente em `[Captures]` desde a US-22.

Só a primeira ocorrência de cada header participa da comparação, sem concatenar
ou separar vírgulas. Ausência faz `==`, `contains` e `exists` falharem, mas aprova
`not contains`; presente vazio satisfaz `exists` e `== ""`.
`contains ""` passa em qualquer valor presente; `not contains ""` falha.

`[Asserts]` exige declaração `HTTP` na entrada. `HTTP *` é aceito com ou sem
assertions/capturas e permite qualquer status, inclusive 404/503. Sua linha mostra
`status aceito por HTTP *`; o resumo conta em `sem_validação`, não em `validadas`.
Isso dispensa **só a comparação numérica**, não headers nem capturas declarados.

Após consumo/fechamento da resposta final, a ordem é status numérico → capturas
provisórias → assertions (headers/JSON/body na ordem escrita) → confirmação,
independentemente da ordem das seções. Uma falha de header retorna 1,
localiza a assertion e esperado/recebido (ausência difere de `""`), não publica
capturas nem envia a próxima requisição. Erro estrutural tardio retorna 2 sem
qualquer envio. Escrita, consumo incompleto, timeout e cancelamento continuam 2.
Valores sensíveis de Authorization/Proxy-Authorization/Cookie/Set-Cookie são
ocultos nos dois lados, inclusive esperados literais; headers comuns usam a
proteção de segredos conhecidos. Havendo captura secreta pendente na mesma
resposta, ambos os operandos são ocultos integralmente, antes de confirmar o lote.
Nenhum body ou coleção de headers é exibido.

Equivalência com Hurl 8.0.0 é limitada aos pares observados: **repetidos são listas
no Hurl**, que também interpola esperados e avalia capturas antes das assertions.
Essas divergências são deliberadas, preservando as regras da história; veja
[matriz de compatibilidade](docs/compatibility.md).

## Validar formato de headers dinâmicos (US-24 / issue #46)

Salve `session.hurl` e execute
`bin/httpgo test session.hurl --var base_url=http://localhost:8080` contra uma
API que devolva `session=<valor variável>; Path=/; HttpOnly; Secure; SameSite=Lax`:

```hurl
GET {{base_url}}/session

HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
```

Exemplo versionado em `testdata/scenarios/scenario_assert_header_matches.hurl`,
executado contra HTTP local nos E2E. O dummy manual não promete esse cookie.
Remover `Secure`, alterar o formato ou omitir o header retorna **1**, sem imprimir
cookie/padrão sensível, publicar capturas ou enviar a próxima requisição.

`header "nome" matches "padrão"` exige `HTTP <status>` ou `HTTP *`. O padrão
é **uma string JSON literal**: escapes JSON são decodificados uma vez antes da
regex Go, como na US-15. Por exemplo, `"\s"` não é escape JSON válido;
use `"\\s"` para a classe regex de whitespace. Operadores de header anteriores
mantêm sua gramática de literais. Sem âncoras, basta encontrar um trecho;
`^`/`$`, classes, grupos, alternativas, repetições, Unicode e opções internas
`(?i)`, `(?m)`, `(?s)` seguem Go, sem normalização Unicode. Não há nova tolerância
para controles inválidos em valores HTTP.

Nomes ignoram caixa; somente a **primeira ocorrência** é comparada, mesmo vazia.
Não concatena valores nem divide vírgulas. Ausência sempre falha, inclusive com
padrão vazio; padrão vazio passa em qualquer valor presente e `^$` somente no
vazio presente. Regex/escape inválido, lookaround, backreference, delimitador sem
aspas ou operando extra retorna **2 antes de qualquer envio do arquivo**.

A ordem permanece resposta final consumida/fechada → status → assertions na ordem
escrita → capturas. Header isolado não retém body nem exige JSON. Leitura/close,
timeout, cancelamento ou escrita de resultado continuam erros **2**, sem retry.
Padrão e real de Authorization, Proxy-Authorization, Cookie e Set-Cookie ficam
integralmente ocultos; headers comuns usam proteção de segredos conhecidos/derivados
e captura secreta pendente. Nenhuma regex compilada é entregue ao relatório.

Placeholders como `{{nome}}` permanecem estáticos, definidos ou não, sujeitos à
sintaxe regex no perfil literal. `--render-asserts` habilita o perfil US-30 acima. Assertions regex de
body/variável e seleção de todas as ocorrências continuam fora do recorte.
Extração regex de body/header é a US-25.
Os 30 pares `header_matches_*` confrontados com Hurl **8.0.0** e as diferenças de
gramática, primeira ocorrência/lista, interpolação, validação e códigos estão na
[matriz diferencial](docs/compatibility.md#regex-de-headers--us-24--issue-46).
Não se anuncia equivalência geral das engines ou homologação da suíte real.

## Validar dados JSON da resposta (US-08 / issue #20)

Salve como `product.hurl` e execute
`bin/httpgo test product.hurl --var base_url=http://localhost:8080` contra uma API
que devolva `{"name":"Pizza","price":4500,"active":true}`:

```hurl
GET {{base_url}}/products/1

HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
jsonpath "$.price" == 4500
jsonpath "$.active" == true
```

O cenário é `testdata/scenarios/scenario_assert_json.hurl`. O fluxo autenticado
com headers, ID de alta precisão, consulta e reutilização tipada está em
`testdata/scenarios/scenario_assert_json_auth_products.hurl`; ambos são executados
contra HTTP real pelos E2E. O dummy manual não promete esse contrato JSON.

**Ampliação intencional:** `jsonpath "consulta" == <literal JSON>` aceita string,
número, booleano, null, array ou objeto, inclusive vazios. JSONPath combina
raiz `$`, propriedades por ponto, índices não negativos, `[*]` de arrays e filtros estáticos por igualdade (US-28). Consultas
têm estrutura estática e operandos opt-in da US-31; no perfil padrão os esperados são literais JSON completos na mesma linha, nunca
templates: `"{{nome}}"` permanece texto mesmo com a variável definida.

A igualdade preserva tipos: `"4500"` difere de `4500`. Números são comparados
exatamente por valor (`1`, `1.0`, `1e0` são iguais), inclusive aninhados e com
expoentes grandes, sem float64 ou expansão de potências. Arrays exigem a mesma
ordem; objetos ignoram somente a ordem das chaves. Valor diferente, propriedade
ausente ou índice fora do array retorna **1**; ausência difere de null presente.
JSON vazio/inválido/adicional ou percurso por tipo incompatível retorna **2**,
como erro de avaliação JSON, não de transporte/captura. Sintaxe ou construção
não suportada em qualquer entrada rejeita o arquivo inteiro antes da rede.

A avaliação é status numérico → headers, JSON e body **na ordem escrita** → capturas.
A primeira falha impede capturas e próximos envios; `HTTP *` dispensa só a
comparação numérica. O JSON da resposta é decodificado sob demanda, portanto um
header divergente anterior prevalece sobre JSON inválido, não o inverso.

Diagnósticos mostram consulta, arquivo/linha/coluna e valores tipados sanitizados,
com ausência distinguível de null. Segredos conhecidos/derivados são protegidos.
Com captura marcada secreta pendente **na mesma resposta**, ambos os operandos de
falhas de headers e JSON ficam integralmente ocultos, inclusive nos metadados do
callback, sem avaliar nem publicar a captura antes da hora. Isso não detecta
segredos arbitrários não marcados. Body, documento e coleções de headers não
são despejados nem acumulados no resultado.

A US-14 acrescenta presença, contagem e limites (abaixo), sem ampliar JSONPath.
A comparação por regex em JSON restringe-se a strings selecionadas (US-15);
headers usam `matches` da US-24 e extração de body/header é o recorte US-25. Consulte
[Validar padrões textuais](docs/subset.md#padrões-textuais--us-15--issue-36). Não há
interpolação estrutural de consultas; operandos de filtros só interpolam com o opt-in US-31 e padrões com o opt-in US-30 acima.
Hurl 8.0.0 rejeita os literais esperados
compostos e o literal com expoente testados; a gramática e os códigos próprios
estão classificados em [compatibilidade](docs/compatibility.md), sem promessa de
equivalência geral.

## Validar estrutura, quantidade e limites (US-14 / issue #34)

Contra uma API que devolva
`{"items":[{"id":null},{"id":2},{"id":3}],"total":3}`, execute
`bin/httpgo test testdata/scenarios/scenario_assert_structure.hurl --var base_url=http://localhost:8080`:

```hurl
GET {{base_url}}/products

HTTP 200
[Asserts]
jsonpath "$.items" count == 3
jsonpath "$.total" >= 1
jsonpath "$.items[0].id" exists
```

| Construção | Contrato |
|---|---|
| `exists` | sem esperado; null, vazios, false e zero presentes existem; ausência falha |
| `count ==`, `count >`, `count >=`, `count <`, `count <=` | tamanho do array selecionado, incluindo elementos null; vazio tem zero; esperado inteiro não negativo, sem limite de máquina |
| `>`, `>=`, `<`, `<=` | dois números JSON, inclusive negativos/decimais; ordem exata, sem float64, coerção ou arredondamento |

Contagem nunca infere tamanho de string/objeto. Representações numericamente
inteiras como `1.0`/`1e3` são aceitas como quantidade, sem impor grafia inteira.
Condição falsa, ausência ou tipo selecionado incompatível retorna **1**; esperado
inválido (inclusive count negativo/fracionário) ou sintaxe inválida retorna **2**
antes de qualquer HTTP. Resposta JSON inválida ou percurso incompatível mantém
**2**. Ordem US-34: status → capturas provisórias → assertions escritas → confirmação; fail-fast preservado.
Diagnósticos incluem operador/causa, consulta e localização; count mostra somente
a quantidade, não a coleção. Segredos conhecidos/derivados continuam sanitizados;
captura secreta pendente oculta integralmente ambos os operandos, inclusive contagens.

O diferencial com **Hurl 8.0.0** confirmou presença de null, vazios, false e zero,
contagem e limites dos pares compartilhados. Há diferenças deliberadas de gramática
(esperados com expoente), pré-validação de count negativo/fracionário e códigos;
veja [matriz](docs/compatibility.md). Count em captura JSONPath está disponível na US-33 acima; não há count em strings/objetos,
novos JSONPaths, filtros, agregações, aritmética ou regex. O dummy manual não promete
essa resposta; o exemplo é testado contra HTTP local pelos E2E.

## Validar padrões textuais (US-15 / issue #36)

Para verificar nomes variáveis sem exigir igualdade literal, use `matches` em
`[Asserts]` (padrão JSON string literal, sintaxe de regex Go):

```hurl
GET {{base_url}}/products/1
HTTP 200
[Asserts]
jsonpath "$.name" matches "^Pizza"
```

`matches` busca trecho; âncoras controlam a string inteira e opções como `(?i)`
são aceitas. Ausência, valor não string ou divergência retorna 1; regex ou
escape inválido retorna 2 antes de qualquer envio. Placeholders no padrão são
literais. A US-24 acrescenta `matches` em headers (acima); `matches` em body/variáveis
continuam não suportados; interpolação de esperados exige o opt-in US-30.
Captura por regex de body/header é o
recorte US-25. A evidência específica frente a Hurl 8.0.0 e
os códigos próprios estão na [matriz](docs/compatibility.md).

## Documentação e desenvolvimento

- [Subconjunto e contratos da CLI](docs/subset.md)
- [Arquitetura e ciclo de vida](docs/architecture.md)
- [Servidor dummy para desenvolvimento](docs/dev-server.md)
- [Compatibilidade diferencial com Hurl 8.0.0](docs/compatibility.md)
- [Comandos, evidências e bloqueios de validação](docs/validation.md)
- [Regras do repositório](AGENTS.md)

```sh
make fmt lint test
# Explicitamente, além do lint:
go vet ./...
make build
make compatibility
# Opcional durante as histórias; build cruzado não comprova execução nativa:
make portable
```

Os testes com `-race` exigem CGO e compilador/headers C no ambiente de
**desenvolvimento**, não no binário entregue. O gate local com race já passou;
a evidência Linux por história está registrada em `docs/validation.md`, separada dos
históricos anteriores. Conforme o plano v4, seção 7, cross-build e testes nativos
macOS/Windows são opcionais nesta história e ficam para a validação final de
portabilidade/release, sem bloquear a entrega. Não foi executado runtime nessas
plataformas nesta história; cross-build não o comprovaria. A aprovação da história
passa pelo gate de revisão; não há alegação de compatibilidade Hurl completa.
