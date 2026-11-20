# Guia de assertions

**Assertion valida; capture extrai.** `[Asserts]` compara propriedades da resposta;
`[Captures]` guarda valores para reutilização. Capturar um ID não valida seu valor
de negócio. Este catálogo consolida as combinações disponíveis nesta revisão;
não implica JSONPath completo nem equivalência integral com Hurl.

Consulte também [configuração](configuration.md), [captures](captures.md) e a
[referência normativa](subset.md). As regras comuns de seleção JSONPath estão
centralizadas em [gramática e cardinalidade](captures.md#jsonpath-tipos-e-seleção),
e as de publicação em [ordem e renovação](captures.md#ordem-escopo-e-renovação).

## Pré-requisitos e primeiro exemplo

Tenha `httpgo` no `PATH` ([instalação](../README.md#instalação)). Os comandos usam
shell POSIX; no PowerShell leia `$LASTEXITCODE` no lugar de `echo $?`.
Use somente dados fictícios, numa pasta de trabalho separada do clone.

Para este primeiro exemplo, execute na raiz do clone
`go run ./tools/dummy/main.go`. O [dummy](dev-server.md) deve ficar aberto em
127.0.0.1:8080. `GET /products/1` retorna 200, Content-Type `application/json`
e `{"id":1,"name":"Pizza","price":4500}`. Salve `validacoes.httpgo`:

```hurl
GET {{base_url}}/products/1
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
jsonpath "$.name" == "Pizza"
jsonpath "$.price" >= 1
body contains "Pizza"
body not contains "passwordHash"
```

```sh
httpgo test validacoes.httpgo --var base_url=http://localhost:8080
echo $?
```

Resultado **0**, status/assertions aprovados em stdout, sem erro em stderr.
Troque apenas `jsonpath "$.name" == "Pizza"` por `jsonpath "$.name" == "Suco"`:
resultado **1**, diagnóstico localizado de divergência, não de transporte.
JSON inválido na resposta seria avaliação **2**. Encerre o dummy com Ctrl+C.

Uma seção `[Asserts]` não pode ser órfã, vazia ou duplicada; exige `HTTP <status>`
ou `HTTP *` na mesma entrada. Pode aparecer antes/depois de `[Captures]`.
Cada declaração ocupa uma linha; comentários somente em linha própria.
O parser valida **o arquivo inteiro antes de HTTP**: sintaxe inválida e
construção reconhecida não suportada são erros distintos, código **2**, com
arquivo/linha/coluna. Valores dependentes de resposta são resolvidos só quando
alcançados. Nada é ignorado em silêncio.

## Status

| Sintaxe no cenário | Resultado para resposta 200 | Limite |
|---|---|---|
| `HTTP 200` | Aprova a comparação numérica | Exatamente três dígitos ASCII, entre 100 e 599; `HTTP 201` falha **1** |
| `HTTP *` | Aceita e informa status, sem comparação numérica | Não dispensa assertions/capturas |
| Omitir a linha `HTTP` | Aceita e informa **sem validação de status** | Não permite `[Asserts]`; capturas ainda são possíveis |

No primeiro exemplo, substitua a linha HTTP ou remova também `[Asserts]` e suas
linhas ao experimentar a omissão. Use o mesmo comando. Cada variante acima
retorna **0** para a resposta indicada. Status 4xx/5xx esperado também pode
aprovar: não é automaticamente erro de transporte. Só a resposta final após
redirects é validada.

## Catálogo JSONPath

### Entrada comum e execução dos exemplos

Para **todas as linhas das tabelas JSONPath**, use uma API de testes em 8081 com
`GET /validation`, resposta **200**, Content-Type `application/json` e este body
completo (o dummy **não** fornece essa rota):

```json
{"name":"Pizza","price":4500,"active":false,"items":[null,2,3],"codes":["REQUIRED","INVALID"],"nullable":null,"empty":"","emptyArray":[],"emptyObject":{},"attempts":3,"decimal":3.0,"exponent":3e0,"zero":0}
```

Salve `catalogo.httpgo`, usando uma ou mais linhas **completas** da primeira
coluna das tabelas no lugar do comentário:

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
# Substitua por uma ou mais assertions do catálogo; a seção não pode ficar vazia.
jsonpath "$.name" == "Pizza"
```

```sh
httpgo test catalogo.httpgo --var base_url=http://localhost:8081
```

Cada exemplo da primeira coluna aprova (**0**) com essa entrada; os contrapontos
indicam divergência **1**, salvo erro **2** explicitamente indicado. A consulta
seleciona **um valor final reduzido**, não aplica o predicado por item.

### Igualdade, desigualdade e pertencimento

| Sintaxe e exemplo executável | Operando aceito | Resultado/contraponto |
|---|---|---|
| `jsonpath "$.name" == "Pizza"` | Um valor JSON completo: string, número, booleano, `null`, array ou objeto | Aprova; esperado `"Suco"` ou tipo diferente reprova **1** |
| `jsonpath "$.name" != "Suco"` | Mesmo contrato JSON de `==` | Aprova; esperado `"Pizza"` reprova **1** |
| `jsonpath "$.codes" contains "REQUIRED"` | Um escalar JSON: string, número, booleano ou `null` | Aprova; `"OTHER_TENANT"` reprova **1** |
| `jsonpath "$.codes" not contains "OTHER_TENANT"` | Um escalar JSON | Aprova; `"REQUIRED"` reprova **1** |

Igualdade é tipada/exata, sem coerção: `4500` difere de `"4500"`, `false` de `0`,
`[]` de `{}`. Números `1`, `1.0` e `1e0` são iguais, sem arredondamento por
float64, inclusive inteiros além de `2^53`/int64 e decimais/expoentes extremos.
Arrays comparam ordem e conteúdo; objetos comparam chaves/conteúdo sem importar
ordem de chaves. `!=` exige valor **presente**: ausência falha **1**, inclusive
contra `null`; `null` presente é valor comparável, não ausência.

Em strings, pertencimento busca substring literal e exige esperado string.
Em arrays, compara **elementos diretos** por tipo/valor, inclusive números
exatos e `null`, sem busca recursiva ou comparação de JSON textual. Arrays/objetos
não são operandos válidos de pertencimento. Array vazio não contém escalar;
toda string contém `""`, inclusive vazia.

Ausência reprova `contains` e aprova `not contains`. Porém número/booleano/objeto/
`null` selecionados, ou string com esperado não string, falham **1 nos dois
sentidos**. Negação não transforma tipo incompatível nem avaliação inválida em
sucesso. String distingue caixa e não normaliza whitespace/Unicode.

Exemplos adicionais sob o mesmo comando/entrada: `jsonpath "$.nullable" == null`,
`jsonpath "$.items" == [null,2,3]`, `jsonpath "$.emptyObject" == {}` e
`jsonpath "$.items" contains null` aprovam **0**; `jsonpath "$.missing" != null`
falha **1**. Um literal inválido/adicional na mesma linha é entrada **2**.

### Presença, tipos e vazios

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `jsonpath "$.nullable" exists` | Nenhum | Aprova até com `null`; `$.missing` reprova **1** |
| `jsonpath "$.missing" not exists` | Nenhum | Aprova ausência; `$.nullable` reprova **1** |
| `jsonpath "$.empty" isString` | Nenhum | Aprova string, inclusive vazia; `$.price` reprova **1** |
| `jsonpath "$.attempts" isInteger` | Nenhum | Aprova número em **notação inteira**; `$.decimal`/`$.exponent` reprovam **1** |
| `jsonpath "$.active" isBoolean` | Nenhum | Aprova `true` e `false`; `$.zero` reprova **1** |
| `jsonpath "$.emptyArray" isEmpty` | Nenhum | Aprova somente `""`, `[]`, `{}`; `$.nullable`/`$.items` reprovam **1** |

Presença inclui vazios, `false`, zero e `null`. Índice fora do array é ausência.
Os quatro predicados de tipo/vazio reprovam ausência **1**. `isInteger` aceita
negativos, `-0` e inteiros sem limite de máquina; não strings numéricas,
decimais `3.0` ou expoentes `3e0`, mesmo sendo matematicamente integrais.
Isso **não** muda igualdade numérica nem contagem integral. Em `isEmpty`,
whitespace é conteúdo; `[null]`, `{"x":null}`, `null`, números/booleanos não passam.
Operando extra é sintaxe inválida **2**. Não há schema, outros predicados de tipo,
`count isInteger` ou negação genérica como `not isString`.

### Comparação numérica

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `jsonpath "$.price" > 4499` | Número JSON | Aprova; `> 4500` reprova **1** |
| `jsonpath "$.price" >= 4500` | Número JSON | Aprova; `>= 4501` reprova **1** |
| `jsonpath "$.price" < 4501` | Número JSON | Aprova; `< 4500` reprova **1** |
| `jsonpath "$.price" <= 4500` | Número JSON | Aprova; `<= 4499` reprova **1** |

Comparações são exatas, incluindo negativos, decimais e expoentes, sem coerção
textual. Seleção ausente/não numérica ou condição falsa falha **1**. Esperado
estático não numérico é entrada **2**, antes de HTTP; dinâmico incompatível é
avaliação **2**. Não há aritmética.

### Contagem de array

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `jsonpath "$.items" count == 3` | Número JSON inteiro não negativo | Aprova; `== 4` reprova **1** |
| `jsonpath "$.items" count > 2` | Mesmo contrato | Aprova; `> 3` reprova **1** |
| `jsonpath "$.items" count >= 3` | Mesmo contrato | Aprova; `>= 4` reprova **1** |
| `jsonpath "$.items" count < 4` | Mesmo contrato | Aprova; `< 3` reprova **1** |
| `jsonpath "$.items" count <= 3` | Mesmo contrato | Aprova; `<= 2` reprova **1** |

Conta só elementos do **array final**, inclusive `null`; array vazio tem zero.
Ausência ou tipo não array falha **1**, diferente de captura `count` (**2**).
Não conta caracteres, propriedades ou correspondências internas de arrays.
Quantidade não tem limite de inteiro de máquina e aceita representações exatas
integrais como `3.0`/`3e0`. Negativo/fracionário estático é entrada **2** antes
de HTTP, dinâmico inválido é avaliação **2**. Não há `count !=`, parâmetros,
`count contains`, pipelines, soma/média ou contagem de header.

Para capturar baseline e compará-lo com uma resposta posterior, veja o
[exemplo completo de contagem](captures.md#capturar-contagem-e-reutilizar-em-assertion):
`jsonpath "$.items" count == {{orders_before}}` exige **`--render-asserts`**.

### Regex em valor JSON selecionado

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `jsonpath "$.name" matches "^Pizza$"` | Uma string JSON com regex Go | Aprova; `"^Suco$"` reprova **1** |

Somente string selecionada participa; ausência/tipo não string falha **1**,
sem serializar arrays/objetos. Padrão vazio corresponde a qualquer string
presente. Regras de regex estão [abaixo](#regex-escapes-e-limites).

### Ausência versus avaliação inválida

Todos os predicados JSONPath usam a [mesma seleção](captures.md#jsonpath-tipos-e-seleção):
zero resultados é ausência, um conserva tipo, vários formam array sem achatamento.
Projeções/filtros não mudam o catálogo de predicados. Um único escalar projetado
não é array para `count`; dois escalares formam array e contam 2. Um array único
continua array, inclusive vazio.

JSON malformado/vazio/com valores adicionais, propriedade em não objeto ou
índice/wildcard/filtro em não array gera **avaliação 2**, até em ramo misto com
ramos válidos e em negação. `$.nullable.id` é percurso incompatível, não ausência.
Isso não é divergência de tipo do valor **final** entregue ao predicado (**1**).

## Catálogo de headers

Para esta tabela, a API de testes em 8081 deve responder `GET /headers` com
**200**, `X-Request-Id: req-123` e `X-Empty:` (valor vazio). O body pode ser vazio,
não precisa ser JSON. Salve `headers.httpgo` com a linha GET, `HTTP 200`,
`[Asserts]` e uma ou mais declarações da tabela:

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `header "X-Request-Id" == "req-123"` | String entre aspas duplas | Aprova **0**; `"req-999"` reprova **1** |
| `header "X-Request-Id" contains "req-"` | String entre aspas duplas | Aprova **0**; `"other"` reprova **1** |
| `header "X-Request-Id" not contains "other"` | String entre aspas duplas | Aprova **0**; `"req-"` reprova **1** |
| `header "X-Request-Id" exists` | Nenhum | Aprova **0**; header ausente reprova **1** |
| `header "X-Request-Id" matches "^req-[0-9]+$"` | Uma string JSON com regex Go | Aprova **0**; `"^other$"` reprova **1** |

```sh
httpgo test headers.httpgo --var base_url=http://localhost:8081
```

O nome é token HTTP estático, consultado sem distinguir caixa; valores distinguem
caixa. **Primeira ocorrência recebida**, inclusive vazia, sem concatenar valores
ou dividir vírgulas. Header presente vazio aprova `exists`, `== ""`, `contains ""`
e `matches "^$"`; reprova `not contains ""`. Ausência reprova `==`, `contains`,
`exists`, `matches` (até padrão vazio), mas aprova `not contains`.

Não há `header !=`, `header not exists`, count, consulta de todas as ocorrências
ou extração regex de header **como query de assertion**. Extração de header é
[capture](captures.md#extrair-cookie-explicitamente), não esse catálogo.

Para testar formato de cookie, uma API em 8081 deve responder `GET /session`
com 200 e `Set-Cookie: session=ficticio; Path=/; HttpOnly; Secure; SameSite=Lax`.
Salve `session.httpgo` ou use a fixture na raiz do clone:

```hurl
GET {{base_url}}/session

HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
```

```sh
httpgo test testdata/scenarios/scenario_assert_header_matches.httpgo --var base_url=http://localhost:8081
```

Resultado **0**; retirar Secure falha **1**. Header sensível oculta padrão e real.

## Catálogo de body textual

API em 8081: `GET /export` retorna **200**, Content-Type `text/plain; charset=utf-8`
e body exato `customer Pizza`. Salve `body.httpgo` com GET, `HTTP 200`,
`[Asserts]` e as linhas abaixo:

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `body contains "customer"` | String entre aspas duplas | Aprova **0**; `"Customer"` reprova **1** |
| `body not contains "passwordHash"` | String entre aspas duplas | Aprova **0**; `"Pizza"` reprova **1** |

```sh
httpgo test body.httpgo --var base_url=http://localhost:8081
```

Busca substring no texto **final completo**, sem reserializar JSON ou normalizar
espaços, caixa, quebras de linha ou Unicode. Todo texto contém `""`, inclusive
body vazio; sua negação sempre falha **1**. Ausência de bytes é texto vazio,
não seleção ausente. UTF-8 estrito por padrão; charset vem do primeiro
Content-Type. Charset desconhecido/conversão inválida é avaliação **2**, inclusive
na negação. ISO-8859-1 segue o índice web (windows-1252, 0x80 → €).

Não há `body ==`, `exists`, `not exists`, `matches` ou count. Para regex, use a
**query de extração** abaixo. Busca literal de `passwordHash` não prova ausência
de todas as codificações possíveis de um segredo.

## Catálogo de extração regex do body

API em 8081: `GET /message/123` retorna **200**, Content-Type
`text/plain; charset=utf-8` e body `Seu código de verificação é: 123456`.
Salve `regex.httpgo` com GET, `HTTP 200`, `[Asserts]` e uma ou mais linhas:

| Sintaxe e exemplo executável | Operando | Resultado/contraponto |
|---|---|---|
| `regex "verificação é: ([0-9]{6})" == "123456"` | Uma string JSON | Aprova **0**; `"654321"` reprova **1** |
| `regex "verificação é: ([0-9]{6})" contains "234"` | Uma string JSON | Aprova **0**; `"999"` reprova **1** |
| `regex "verificação é: ([0-9]{6})" not contains "999"` | Uma string JSON | Aprova **0**; `"234"` reprova **1** |
| `regex "verificação é: ([0-9]{6})" matches "^[0-9]{6}$"` | Uma string JSON com regex Go | Aprova **0**; `"^[0-9]{7}$"` reprova **1** |
| `regex "verificação é: ([0-9]{6})" exists` | Nenhum | Aprova **0**; sem match/grupo participante reprova **1** |

```sh
httpgo test regex.httpgo --var base_url=http://localhost:8081
```

Extrai **somente o primeiro grupo da primeira correspondência** do body
final completo, com a mesma decodificação textual de body. Não publica variável,
não altera capturas, não seleciona a correspondência inteira nem grupo posterior.
Padrão de extração é **sempre estático**, até com `--render-asserts`.

| Seleção | `==`, `contains`, `matches` | `exists` | `not contains` |
|---|---|---|---|
| Grupo presente | Compara como string, sem coerção | Aprova | Nega substring |
| Sem match ou primeiro grupo opcional não participante | Falha **1** | Falha **1** | Aprova |
| Grupo participante vazio | `== ""`, `contains ""`, `matches "^$"` aprovam | Aprova | `not contains ""` falha **1** |

Regex válida **sem nenhum grupo** e body indecodificável são avaliação **2**,
também na negação. Em **captura** regex, ausência de match/grupo participante
é **2**, não divergência **1**. Não há `!=`, `not exists`, escolha de grupo,
pipelines, extração dinâmica ou query regex de header em assertion.

### Capturar e validar o código na mesma resposta

Use a API de mensagem acima e `POST /verify` aceitando `{"code":"123456"}` com
200. Salve `verificacao.httpgo` ou use a fixture:

```hurl
GET {{mailbox_url}}/message/123
HTTP 200
[Captures]
signup_code: regex "verificação é: ([0-9]{6})"
[Asserts]
regex "verificação é: ([0-9]{6})" matches "^[0-9]{6}$"

POST {{base_url}}/verify
Content-Type: application/json
{"code":"{{signup_code}}"}
HTTP 200
```

```sh
httpgo test testdata/scenarios/scenario_regex_assertion_signup.httpgo --var mailbox_url=http://localhost:8081 --var base_url=http://localhost:8081 --secret signup_code
```

Resultado **0**, dois envios, sem revelar o código. Exigir sete dígitos só no
esperado da assertion falha **1**, sem confirmação/POST; sem correspondência,
a captura falha **2** antes da assertion.

## Regex: escapes e limites

`matches` recebe exatamente uma string **JSON** entre aspas duplas: `"\\d"`
entrega `\d` à regex; `"\x41"` é inválido como JSON. Queries/capturas de extração
também usam string JSON para seus padrões. Esperados comuns de header/body são
strings entre aspas duplas (o leitor legado usa escapes de string Go); para
exemplos portáveis use escapes comuns, como `\"`, `\\`, `\n`.
Não há delimitadores `/.../` ou operandos adicionais.

Regex segue `regexp` Go: busca por trecho, distinguindo caixa, sem normalização
Unicode. Aceita classes, grupos, alternativas, repetições e opções internas
`(?i)`, `(?m)`, `(?s)`. Use `^`/`$` para exigir limites; ponto não cruza newline
sem `(?s)`. Lookaround e backreferences não são suportados.

Padrão estático malformado/não suportado impede todo HTTP (**2**), mesmo em
entrada tardia. Padrão `matches` com referências, habilitado pelo opt-in abaixo,
só é compilado quando alcançado; inválido é avaliação **2**, antes da consulta.
Metacaracteres inseridos **não** recebem escape automático.

## Perfil literal e --render-asserts

Sem flag, esperados são **literais**: `"{{nome}}"` significa esse texto, mesmo
com variável definida. Regex continua sujeita à sua gramática, não ganha escape
automático. `jsonpath "$.id" == {{id}}` não é literal JSON e é entrada **2** antes
de HTTP. Placeholders em **consultas JSONPath**, inclusive em strings de filtros,
são erro estático sem a flag.

`--render-asserts` é um **opt-in HTTPGo por invocação**, sem valor. Ele resolve
esperados de **todos os operadores que recebem operando** do catálogo, mantendo
suas restrições; predicados sem operando continuam sem operando. Pode usar
iniciais, confirmados anteriores, atribuições literais de `[Options]` já alcançadas
e capturas provisórias válidas da resposta atual.

`variable: nome=literal` aplica antes da própria requisição, persiste no arquivo
e pode ser renovada por captura confirmada. Não habilita renderização sozinha:
sem `--render-asserts`, esperados continuam literais e filtros dinâmicos falham
antes de HTTP. O valor atribuído não é expandido recursivamente; arrays/objetos
escritos na opção são strings. A query de assertion `variable` continua não
suportada. Veja [atribuições e tipos](subset.md#atribuições-literais-por-entrada--us-44--issue-95).

| Contexto esperado | Exemplo com opt-in | Tipo exigido |
|---|---|---|
| Igualdade/desigualdade JSON | `jsonpath "$.id" == {{id}}` ou `!= {{old_id}}` | Qualquer valor JSON completo, tipo preservado |
| Comparação numérica | `jsonpath "$.price" >= {{minimum}}` | Número |
| Count | `jsonpath "$.items" count == {{baseline}}` | Número inteiro não negativo |
| Pertencimento JSON | `jsonpath "$.codes" contains {{code}}` (também `not contains`) | Escalar, conforme tipo selecionado |
| Header/body/regex e `matches` | `header "X-Order" contains "{{order_number}}"`, `body not contains "{{forbidden}}"`, `regex "code=([0-9]+)" == "{{code}}"`, `jsonpath "$.name" matches "^{{pattern}}$"` | Sempre string entre aspas; composição só escalar |

Valor JSON completo sem aspas preserva tipo/precisão, inclusive arrays/objetos
onde admitidos. Dentro de string, escalares compõem texto escapado;
`"{{id}}"` seria string, diferente de número `1`. Arrays/objetos em texto são
**2**, não serialização implícita. Substituição não é recursiva. As mesmas regras
valem para valores JSON aninhados de igualdade/desigualdade.

Na assertion alcançada, prepara/valida o esperado **antes dos operandos da consulta
e antes de selecionar/decodificar a resposta**. Variável ausente, tipo incompatível
ou regex final inválida é avaliação **2**, nunca aprovação por negação/ausência.
Status/assertion anterior divergente impede resolução de posteriores. Só o operando
de igualdade de filtros pode variar; veja [operandos dinâmicos](captures.md#operandos-dinâmicos-opt-in).
A flag não habilita estrutura de consulta dinâmica, header dinâmico, novos
predicados nem padrão de **extração/captura** dinâmico.

### Comparar com valores anteriores

API em 8081: `POST /orders` e `GET /orders/9007199254740993` retornam 200 com
`{"number":9007199254740993,"name":"Pizza"}`. Salve `orders.httpgo` ou use a fixture:

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

```sh
httpgo test testdata/scenarios/scenario_render_asserts.httpgo --render-asserts --var base_url=http://localhost:8081
```

Resultado **0**, dois envios, preservando precisão. Mudar só o número recebido
no GET falha **1**; sem flag, esperado bare rejeita o arquivo com **2**, sem HTTP.

### Desigualdade e rotação de token

API em 8081: `POST /session/login` retorna 200 e
`{"accessToken":"antigo-ficticio"}`; `POST /session/renew`, 200 e
`{"accessToken":"novo-ficticio"}`; `GET /next` aceita novo Bearer e retorna 200.
Salve `rotacao.httpgo` ou use a fixture:

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

```sh
httpgo test testdata/scenarios/scenario_json_inequality_rotation.httpgo --render-asserts --secret old_token --secret new_token --var base_url=http://localhost:8081
```

Resultado **0**; repetir token falha **1**, sem GET. Token ausente na renovação
faz a **captura** falhar **2 antes da assertion**, sem GET. Sem essa captura, a
assertion `!=` sozinha reprovaria ausência com **1**. Diferir não comprova
revogação do antigo nem validade do novo para outras operações.

## Ordem, falhas e sigilo

Ordem vigente: **resposta completa consumida/fechada → status → todas as capturas
provisórias → assertions na ordem escrita → confirmação atômica com contexto ativo**.
A ordem das seções não muda isso. Status divergente é barreira **1** antes de
capturas; captura inválida é **2** antes de qualquer assertion. Assertions veem
o lote atual válido, mas capturas não encadeiam consultas no próprio lote.

A primeira assertion divergente/avaliação inválida interrompe o arquivo,
descarta lote e impede próximos envios. Sem capturas, header divergente anterior
a JSON inválido dá **1**; invertendo a ordem, JSON dá avaliação **2**. Não se
reavalia assertion posterior só para mudar o código. Sem rollback de efeitos
na API. [Polling opt-in](subset.md#polling-local-por-entrada) retenta só
divergência **1**, usando valores confirmados antes da entrada; erro **2** encerra.

| Código | Significado |
|---|---|
| `0` | Todas as validações declaradas aprovadas |
| `1` | Somente divergência de assertion/status |
| `2` | Uso, entrada, configuração, captura, renderização, avaliação, transporte, cancelamento ou escrita |

Em suíte, **2 prevalece sobre 1**. Falha de saída/escrita curta/close tem
prioridade **2**, sem reenvio para gerar relatório ou anúncio falso de sucesso.
Stdout contém resultados, stderr diagnósticos. Capturas pertencem só ao arquivo.

Diagnósticos identificam consulta, predicado, posição e causa seguros.
Bodies/grupos regex/coleções de headers não são impressos; seleções compostas
são resumidas, não despejadas. Esperado/real escalar só aparece sanitizado.
Authorization, Proxy-Authorization, Cookie e Set-Cookie ocultam ambos os lados;
`--secret nome` e `secrets` protegem também capturas futuras. Captura secreta
pendente oculta operandos sem confirmar lote; marcas/valores conhecidos e derivados
permanecem protegidos após descarte/renovação e troca de arquivo na invocação.
Resultados/callbacks/JUnit não retêm templates, padrões brutos ou regex compiladas.
Stderr já emitido não pode ser sanitizado retroativamente por segredo futuro.

Marque segredos arbitrários explicitamente: não há detecção universal de conteúdo
não marcado ou transformações/codificações desconhecidas. Credenciais iniciais
reais vêm do [ambiente do processo](configuration.md#referências-ao-ambiente-do-processo-e-segredos),
nunca de flags. Proteção de saída não protege argumentos/histórico ou disco.

## Receitas e limites relacionados

- [Projeções, filtros estáticos/dinâmicos](captures.md#jsonpath-tipos-e-seleção):
  mesma seleção tipada e mesmo catálogo de predicados, sem JSONPath completo.
- [Captura de contagem e comparação](captures.md#capturar-contagem-e-reutilizar-em-assertion):
  baseline exato, quantidade não garante conteúdo igual.
- [Inclusão e ausência](subset.md#inclusão-e-ausência-jsonheader) e
  [tipos e vazios](subset.md#tipos-e-valores-vazios-json): contratos detalhados.
- [Compatibilidade e diferenças](compatibility.md): extensão de arquivo não
  altera linguagem; não há query `variable`, XPath, negação genérica ou predicados
  adicionais. Sintaxe aceita não promete equivalência integral com Hurl.

Fixtures adicionais executáveis, com API de testes em 8081 (não o dummy):
`GET /validation`, 200 com `{"codes":["REQUIRED","INVALID"],"name":"Pizza"}` e
Set-Cookie sem Domain, para o primeiro cenário; `GET /session-info`, 200 com
`{"token":"abc","attempts":3,"active":false,"issues":[]}` para o segundo.

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.codes" contains "REQUIRED"
jsonpath "$.codes" not contains "OTHER_TENANT"
jsonpath "$.passwordHash" not exists
header "Set-Cookie" not contains "Domain"
```

```hurl
GET {{base_url}}/session-info
HTTP 200
[Asserts]
jsonpath "$.token" isString
jsonpath "$.attempts" isInteger
jsonpath "$.active" isBoolean
jsonpath "$.issues" isEmpty
```

```sh
httpgo test testdata/scenarios/scenario_assert_membership.httpgo --var base_url=http://localhost:8081
httpgo test testdata/scenarios/json_types.httpgo --var base_url=http://localhost:8081
```

Ambos retornam **0**; acrescentar OTHER_TENANT ou mudar attempts para `3.0`
reprova o respectivo cenário com **1**. Execute fixtures individualmente, não
toda `testdata/scenarios` (contém negativos intencionais).
