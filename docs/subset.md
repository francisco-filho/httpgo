# Subconjunto suportado e extensões — US-01 a US-10 e US-20 (issues #4, #6, #11, #13, #14, #16, #18, #20, #22, #24 e #25)

Este documento define somente o que o binário implementa nesta história.
Não implica suporte ao formato Hurl completo.

## Gramática

Um arquivo UTF-8 contém ao menos uma request-line
`<método> <URL absoluta HTTP/HTTPS>`. Métodos aceitos (em maiúsculas):
`GET`, `POST`, `PUT`, `PATCH` e `DELETE`, combináveis em sequência.
A expectativa opcional `HTTP <status>` ou `HTTP *` associa-se à requisição anterior,
uma única vez. O status numérico tem exatamente três dígitos ASCII, entre 100 e 599.
`HTTP *` aceita qualquer status recebido, inclusive 4xx/5xx, sem contar como
comparação numérica aprovada. A saída o distingue de uma expectativa ausente.
São aceitos espaços/tabs de indentação, linhas vazias, LF/CRLF e comentários em
linha própria iniciados por `#`. Comentários e linhas vazias não desfazem a
associação entre requisição e HTTP. A request-line seguinte inicia outra entrada.

```hurl
# A expectativa pertence ao GET anterior.
GET https://example.test/health
HTTP 200

# Sem expectativa: executa, mas NÃO valida status.
GET https://example.test/error
```

URLs são verificadas estruturalmente antes da rede: esquema HTTP/HTTPS, host não
vazio, IPv6 entre colchetes (inclusive zona percent-encoded) e porta numérica de
0 a 65535 quando declarada. Isso não comprova resolução DNS, conexão ou
existência do endpoint. Scheme é normalizado sem distinção de caixa.

O parser valida **todo** o arquivo antes de retornar um cenário executável.
Arquivo vazio/com só comentários, URL inválida, status inválido, expectativa
órfã/duplicada e instrução desconhecida são erros de entrada. Nada do arquivo é
enviado nesses casos, mesmo quando o erro aparece depois de GETs válidos.

### Upload multipart básico — US-20 (issue #24)

`POST`, `PUT` e `PATCH` aceitam `[MultipartFormData]` após os headers,
com exatamente um arquivo e zero ou mais campos textuais, em ordem:

```hurl
POST http://localhost:8080/documents
[MultipartFormData]
description: Documento de teste
document: file,../fixtures/document.pdf; application/pdf
HTTP 201
```

O formulário é exclusivo de outros bodies; nomes estáticos não se repetem.
O arquivo mantém seus bytes (inclusive vazio, binário e placeholders literais),
mesmo com a extensão `--render-files` ativa,
usa somente o basename como filename e é enviado incrementalmente, sem buffer
integral. O runner abre a fonte na vez da entrada e a fecha antes do resultado.
O transporte gera boundary e Content-Type coerentes; um header explícito deve
ser `multipart/form-data`, sem boundary manual. Outros parâmetros válidos, como
`charset=utf-8`, são preservados ao completar o header. Caminhos seguem as regras de arquivo
externo abaixo. Conteúdo e descrição operacional não entram nos resultados.
Somente valores textuais interpolam, sem recursão ou interpretação JSON;
arrays/objetos não são convertidos em texto e valores derivados de segredos
continuam protegidos. Nomes, MIME e caminho são estáticos. Controles em nomes,
filename e MIME são rejeitados antes da rede. MIME explícito prevalece; sem ele,
a extensão (sem distinção de caixa) usa a tabela determinística abaixo, sem
consultar o sistema nem examinar os bytes:

| Extensão | MIME |
|---|---|
| `.pdf` | `application/pdf` |
| `.json` | `application/json` |
| `.xml` | `application/xml` |
| `.txt` | `text/plain` |
| `.csv` | `text/csv` |
| `.html`, `.htm` | `text/html` |
| `.png` | `image/png` |
| `.jpg`, `.jpeg` | `image/jpeg` |
| `.gif` | `image/gif` |
| `.webp` | `image/webp` |
| `.zip` | `application/zip` |
| demais ou sem extensão | `application/octet-stream` |

Estrutura/metadados estáticos inválidos impedem todos os envios. Variável
ausente, tipo textual inválido, Content-Type dependente de variável proibido ou
falha de arquivo impedem somente a entrada afetada e as seguintes (código 2),
sem contar envio inexistente nem desfazer resultados anteriores.

Não há filename customizado, headers arbitrários por parte, múltiplos arquivos,
campos repetidos, partes JSON tipadas, multipart aninhado, renderização do arquivo,
retomada ou progresso. MIME é literal válido `tipo/subtipo` (parâmetros MIME
válidos são preservados), nunca template. `document` não é palavra reservada.
Nomes de campos aceitam texto estático sem controles ou espaços externos.
Valores textuais preservam os espaços após remover a indentação inicial após `:`;
espaços finais permanecem. Linhas vazias e comentários próprios separam instruções,
não criam campos multilinha. Todos os campos são validados antes de abrir o arquivo.

307/308 rebobinam a mesma fonte e reutilizam boundary; 301/302/303 convertem em GET
sem body nem Content-Type multipart. Mantêm-se limite de dez redirects de mesma
origem, TLS verificado, prazo único de dez segundos e nenhum retry. Resposta 413
completa é avaliada normalmente, inclusive antecipada; erro de conexão sem resposta
completa é transporte (2). Leituras HTTP terminam antes de rebobinar/fechar a fonte;
falhas de leitura/fechamento não viram sucesso. I/O local bloqueado pelo SO continua
sujeito à limitação descrita na US-09.

**Mudança de contrato:** `[MultipartFormData]` antes rejeitado agora é suportado
neste recorte. O antigo negativo global virou `unsupported_multipart_get.hurl`;
negativos específicos cobrem limites. Dez pares diferenciais e diferenças Hurl
8.0.0 (incluindo filename e nomes com espaços) estão em `compatibility.md`.

### Headers e JSON inline

Depois da request-line, podem ser declarados headers `Nome: valor`, antes do
body e da expectativa. Nomes usam os tokens ASCII de HTTP (sem espaço antes de
`:`); valores têm espaços/tabs externos removidos e não podem conter CR, LF ou
outros controles, salvo tab. LF/CRLF delimitam linhas do arquivo, não fazem parte
do valor. Continuação de header em outra linha não é suportada. Headers repetidos
são enviados como valores repetidos; `Host` deve ser único e um host/porta válido,
definindo o host HTTP sem alterar a URL operacional nem a política de origem. Headers de enquadramento `Content-Length`,
`Transfer-Encoding` e `Trailer` são explicitamente não suportados: o transporte
calcula o tamanho do body. Não há autenticação ou cookie jar implícito.

`POST`, `PUT`, `PATCH` e `DELETE` aceitam **nenhum body, um único valor JSON ou
uma referência externa** (abaixo). POST/PUT/PATCH também aceitam o multipart
exclusivo descrito acima; DELETE não aceita multipart. Para JSON inline:
objeto, array, string, número, booleano ou `null`, inclusive multilinha. `GET`
permanece sem body. O valor começa no primeiro caractere JSON e termina no último.
Espaços/linhas separadores externos não são enviados; todos os bytes internos,
inclusive indentação, escapes e LF/CRLF, são preservados, sem nova serialização
em bodies estáticos. Bodies com templates substituem apenas os intervalos dos
placeholders, respeitando os contextos descritos abaixo.
Na última linha do valor só pode haver whitespace após ele. A expectativa e a
próxima request-line devem começar em outra linha. Texto dentro de strings JSON
(como `HTTP 200`, `GET`, `#` e `[Asserts]`) não é delimitador externo.

```hurl
POST http://localhost:8080/products
Content-Type: application/json
{"name":"Pizza","price":4500}

HTTP 201

DELETE http://localhost:8080/products/1
HTTP 204
```

Declare `Content-Type: application/json` quando a API exigir: httpgo não o infere.
Body duplicado, JSON inválido, header inválido ou header após o body/expectativa
rejeita **todo o arquivo**, inclusive requisições anteriores válidas.

Construções reconhecidas e não suportadas são diagnosticadas separadamente de
sintaxe inválida. Não são aceitos: HEAD/OPTIONS/CONNECT/TRACE, body em GET,
templates em chaves JSON e nomes de headers, seções além de `[Captures]`,
`[Asserts]` e `[MultipartFormData]`, form-urlencoded e multipart avançado, bodies binários/string multilinha
não JSON, versões como `HTTP/1.1` na expectativa ou comentários após instruções.
O texto bruto não é ecoado. O corpus negativo está em
`testdata/scenarios/unsupported_*.hurl` e `invalid_*.hurl`.

### Body de arquivo externo — US-09 (issue #22)

Alternativamente ao JSON inline, `POST`, `PUT`, `PATCH` e `DELETE` aceitam
`file,<caminho>;` como body completo. O caminho é literal entre `file,` e o
`;` final, incluindo espaços, aspas e barras, sem escapes nem interpolação.
Vírgula e ponto e vírgula no nome não são aceitos. GET continua sem body;
body duplicado/misturado ou header após body rejeita todo o cenário.
Caminhos relativos partem do diretório do `.hurl`, não do diretório corrente;
absolutos são aceitos. Não há sandbox: `..` e absolutos alcançam arquivos fora
da raiz do projeto, usando as permissões do usuário.

```hurl
POST http://localhost:8080/products
Content-Type: application/json
file,../payloads/product.json;
HTTP 201
```

Sem `--render-files`, o arquivo é lido somente na vez da requisição e enviado
integralmente, sem validar JSON, renderizar placeholders ou normalizar bytes
(inclusive vazio e binário). Não se infere Content-Type. Falha de leitura interrompe antes do envio
com código 2; requisições anteriores não são desfeitas. Inexistente, diretório e
acesso negado têm causas controladas distintas; nenhuma causa bruta, conteúdo ou
caminho operacional é impresso. Erros de leitura/fechamento descartam bytes parciais.
A referência é identificada pelo arquivo/linha/coluna do cenário. A proteção não
inspeciona conteúdo para descobrir segredos arbitrários. O arquivo corrente é
carregado integralmente em memória, sem limite de tamanho. Contexto é checado
antes/depois da leitura; o timeout do transporte não interrompe um I/O local
bloqueado no sistema operacional. Multipart usa a variante incremental separada
descrita acima; não converte o body externo completo em streaming.
**Mudança de contrato:** a referência externa, antes rejeitada, passa a ser aceita.

## Extensão HTTPGo: JSON externo com --render-files — US-10 (issue #25)

Esta extensão é **separada do subconjunto compatível com Hurl**. Não se promete
renderização externa equivalente pelo Hurl. Ative por execução, antes ou depois
do arquivo após `test` (flag sem valor; `--render-files=false` não é aceito):

```sh
httpgo test tests/products.hurl --render-files \
  --var product.name=Pizza --var product.price=45 --var product.active=true
```

No payload `payloads/product.json` referenciado pelo exemplo acima:

```json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}}}
```

A flag trata **todos os arquivos usados como body completo** como templates JSON,
mesmo sem placeholders, sem seleção por extensão/Content-Type nem inferência de
headers. **Arquivos multipart permanecem literais**, inclusive vazios/binários e
placeholders indefinidos. Não altera bodies inline ou o caminho literal da US-09.
Sem flag, o contrato bytes da seção anterior permanece integralmente inalterado.

Reutiliza as regras do JSON contextual inline: nomes pontuados explícitos, tipos
preservados em valores completos (inclusive raiz), escalares escapados em strings,
arrays/objetos não convertidos em texto e nenhuma reinterpretação do conteúdo
inserido. Chaves e caminhos não interpolam; sem novas expressões, YAML ou fontes
de variáveis/segredos. Variáveis iniciais e capturas anteriores usam a mesma sessão.
JSON estático válido conserva todos os bytes, inclusive whitespace externo LF/CRLF.

O parser valida o `.hurl` inteiro antes da rede **sem abrir payloads**. Ler,
validar e renderizar o conteúdo externo ocorre só na entrada corrente, antes do
executor e depois de fechar o arquivo. Exige um único valor JSON completo UTF-8.
Vazio, somente whitespace, não JSON, conteúdo adicional, template malformado,
variável ausente, tipo incompatível ou JSON final inválido produz **erro de
renderização (2)**. A entrada afetada e seguintes não são enviadas nem contadas
como iniciadas; resultados anteriores permanecem. Estrutura inválida do `.hurl`
continua rejeitando todos os envios. Abertura/leitura/fechamento mantêm FileError
com posição da referência no cenário, categoria distinta de conteúdo/renderização.

Diagnósticos identificam payload e causa controlada, com linha/coluna em bytes do
conteúdo **original** (LF/CRLF), nunca do JSON expandido. Sem offset confiável,
usam 1:1; incluem a referência `.hurl`. Nomes são sanitizados e exibidos com quoting,
sem despejar caminho absoluto operacional calculado, causas brutas ou payload.
Resultados/callbacks não retêm corpos, planos ou referências operacionais.
Segredos marcados, capturas secretas e derivados/escapes conhecidos permanecem
protegidos em sucesso/falha, sem descobrir segredos arbitrários do arquivo.
Permanecem os limites de memória, acesso local sem sandbox, contexto e I/O bloqueado
já descritos; nenhuma nova goroutine/retry/limite de tamanho é introduzida.

## Variáveis tipadas e templates — US-03/US-04

`--var nome=valor` e `--secret nome` são repetíveis (também aceitam
`--var=nome=valor` e `--secret=nome`). Flags podem vir antes/depois do único
arquivo, após `test`. Cada execução cria contexto novo, sem estado compartilhado.
O primeiro `=` de uma atribuição separa nome e valor; os demais pertencem ao
valor. Texto vazio é definido e difere de ausência. **Mudança intencional de contrato
na US-04:** literal JSON válido preserva seu tipo; valor não JSON permanece texto.
Strings JSON perdem somente as aspas delimitadoras e interpretam escapes; por
exemplo `--var 'code="001"'` força string. `001` sem aspas também é texto (não é
número JSON válido). Números mantêm representação numérica JSON sem passar por
float64, inclusive dentro de arrays/objetos. A tipagem CLI é extensão HTTPGo.

Cada segmento do nome corresponde a `[A-Za-z_][A-Za-z0-9_]*`; pontos separam
segmentos hierárquicos (`api.baseUrl`). Segmento vazio, nome inválido, atribuição
sem `=`, repetição (mesmo valor idêntico) ou conflito ancestral/descendente em qualquer
ordem e tipo impede todos os envios. Nunca há merge ou sobrescrita silenciosa.
Objetos são indivisíveis: `--var 'product={"name":"Pizza"}'` define apenas `product`,
não `product.name`. Nomes pontuados funcionam quando declarados explicitamente.

URL e **valores** de headers aceitam `{{nome}}` e `{{ nome }}`, com espaços/tabs
internos opcionais e múltiplas referências combinadas com texto. O conteúdo
inserido não é interpretado novamente como template. Nomes de headers são
estáticos. URL e headers aceitam apenas escalares: strings usam seu conteúdo,
números sua representação JSON, booleanos `true`/`false` e null o texto `null`.
Arrays/objetos nesses contextos geram erro, nunca conversão implícita.

```hurl
GET {{ api.baseUrl }}/products/{{id}}
Authorization: Bearer {{token}}
X-Probe: prefix-{{id}}-{{empty}}
HTTP 200
```

```sh
httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var id=1 --var empty= --var token=ficticio --secret token
```

Sintaxe de placeholders, nomes e estrutura do **arquivo completo** são validados
antes da rede, com posições de origem. Espaços na request-line só são aceitos
na separação método/URL e dentro de placeholders; comentários inline continuam
inválidos. URLs e headers estáticos mantêm as validações anteriores. Os templates
originais são conservados e resolvidos imediatamente antes da requisição.

Variável ausente, tipo incompatível, JSON final inválido, URL final inválida ou
header final inválido gera **erro de
renderização**, código 2, preserva resultados anteriores e impede a requisição
afetada e as seguintes. A entrada afetada é apresentada sem status e com URL
não renderizada; não é chamada de erro de transporte nem contada como iniciada.
A URL final deve ser absoluta HTTP/HTTPS, com host/porta/encoding válidos; não
há percent-encoding automático, inclusive para espaços, Unicode ou controles.
Forneça o encoding necessário no cenário/valor (por exemplo `a%2Fb`); ele não é
duplicado. Caracteres reservados já válidos conservam sua função, não são
transformados em dados de path/query automaticamente. Valores finais de headers
não podem conter CR/LF ou controles indevidos; `Host` mantém suas restrições.
Os bytes do JSON estático não são alterados pela renderização de URL/headers.

`--secret nome` marca proteção sem fornecer valor. A declaração pode preceder ou
seguir a atribuição e pode marcar um nome ainda ausente, inclusive uma captura
futura; usar esse nome antes de estar disponível continua falhando. Um valor
vazio explicitamente fornecido existe e sua proteção não substitui texto vazio
globalmente. Valores derivados conservam proteção, inclusive path e header comum.

**Credenciais reais em argumentos são proibidas, inclusive em `--var`.** Apenas
valores não sigilosos/fictícios são adequados aqui. A fonte segura pelo ambiente
do processo chega na US-13; não há entrada de segredos por stdin, configuração
YAML nesta entrega. Capturas de respostas não são uma nova fonte de
credenciais iniciais. Proteção de streams não protege histórico do shell,
listagem de processos, arquivos em disco nem segredos arbitrários não marcados.

### JSON contextual — US-04

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}
HTTP 201
```

```sh
httpgo test products.hurl --var api.baseUrl=http://localhost:8080 \
  --var product.name=Pizza --var product.price=4500 \
  --var product.active=true --var 'product.code="001"'
```

- Sem aspas, placeholder deve ocupar **um valor completo**, inclusive raiz.
  Insere string, número, booleano, null, array ou objeto com seu tipo, sem converter
  estruturas em strings. Ex.: `{"value":{{x}}}`, `[{{x}}]` ou `{{x}}` na raiz.
- Dentro de string JSON, placeholders podem compor texto e aceitam só escalares
  segundo as representações de URL/headers. O conteúdo é escapado uma vez, com
  aspas, barras invertidas, controles e Unicode corretos. Escapes estáticos
  existentes no template conservam sua função.
- Valores inseridos nunca são reinterpretados como template, mesmo em estruturas.
  `null` completo é null JSON; `"{{x}}"` com x=null é string `"null"`; x vazio
  completo é string vazia, não ausência. Não há navegação de propriedades,
  expressões/funções ou interpolação de nomes de propriedades.
- Chaves são estáticas, com ou sem aspas ao redor do placeholder. Template
  malformado ou estrutura JSON inválida em qualquer entrada rejeita **todo** o
  arquivo antes da rede. Fechamentos adjacentes (`{"a":{}}`) não são placeholders.
- JSON final é validado novamente antes do executor. Falha impede a requisição
  afetada/posteriores e mantém resultados prévios e código 2. Diagnósticos mapeiam
  offsets para o template original, inclusive multiline/LF/CRLF, nunca posições
  deslocadas pelo escape do valor. Nenhum payload completo é reproduzido.
- Sem placeholders, bytes e validação permanecem os da US-02. A expectativa e a
  entrada seguinte começam em nova linha, inclusive depois de placeholder raiz.

## Capturas tipadas — US-05/US-06

```hurl
POST {{base_url}}/auth/login
Content-Type: application/json
{"username":"{{username}}","password":"{{password}}"}
HTTP 200
[Captures]
auth.token: jsonpath "$.accessToken"

GET {{base_url}}/products
Authorization: Bearer {{auth.token}}
HTTP 200
```

`[Captures]` associa declarações à resposta anterior, após a expectativa quando
presente; também pode existir sem `HTTP`. A seção não pode ser órfã, duplicada
na entrada ou vazia. Cada declaração tem a forma `nome: jsonpath "consulta"`.
Comentários em linha própria e LF/CRLF mantêm a associação. A próxima request-line
encerra a seção; `[Asserts]` também pode sucedê-la na mesma entrada. Outras queries
e filtros de captura não são aceitos nem ignorados.
Nomes seguem a mesma regra ASCII pontuada das variáveis.

O subconjunto de JSONPath aceita `$`, propriedades por ponto e índices decimais
não negativos, combináveis: `$.accessToken`, `$.product.id`, `$.items[0].token` e
`$[0]`. Propriedades simples usam letras ASCII, dígitos e `_`; nomes com outros
caracteres exigiriam uma sintaxe ainda não suportada. Não há wildcard, filtros,
descida recursiva, slices, união, propriedades entre colchetes ou índices negativos.
Consultas inválidas/não suportadas e nomes inválidos em entrada tardia rejeitam
**todo** o arquivo, com arquivo/linha/coluna originais antes de qualquer rede.

A resposta final deve conter exatamente um valor JSON válido. **Ampliação
intencional da US-06:** a seleção aceita string, número, booleano, `null`, array
ou objeto, inclusive na raiz. Campo ausente/índice fora do array ou percurso
incompatível é erro; `null`, string vazia e coleções vazias existem e são válidos.
JSON inválido ou com valores adicionais também falha, sem publicar lote parcial.

Em valor JSON completo, inclusive raiz de body inline ou externo com `--render-files`, o tipo e o conteúdo são
preservados. Números não passam por float64 e mantêm precisão, também aninhados:
`9007199254740993` e `1.2300e+99` não sofrem arredondamento. A string decodificada
conserva seu tipo: `"123"`/`"null"` recebidos não viram número/null. Conteúdo
semelhante a placeholder não é reinterpretado, nem dentro de estruturas.

Em URL, valor de header ou string JSON, strings usam seu conteúdo, números sua
representação JSON, booleanos `true`/`false` e `null` o texto `null`. Arrays/objetos
exigem valor JSON completo; uso textual falha com código 2 e posição no template
original, inclusive LF/CRLF, sem enviar a entrada afetada ou as seguintes.
Escapes JSON, validação da URL final, encoding explícito não duplicado e rejeição
de controles indevidos em headers são os mesmos das variáveis iniciais.
Capturar um objeto não cria variáveis para suas propriedades; uma declaração
separada exige nome não conflitante (por exemplo `productId`, não `product.id`
quando `product` já existe).

Exemplo mínimo (`scenario_capture_products.hurl`):

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

Execute `httpgo test products.hurl --var base_url=http://localhost:8080` contra
API que devolva `{"id":9007199254740993}`. O ID chega exato no path e como número
no body seguinte; os três status são validados. O cenário autenticado completo
está em `scenario_capture_auth_products.hurl` e é exercitado pelos E2E.

Após consumir e fechar a resposta, o status numérico e as assertions de headers/JSON
são validados **antes** de avaliar capturas, independentemente da ordem das seções.
Entre assertions, vale a ordem escrita: um header divergente anterior ao JSON
inválido retorna 1; invertendo a ordem, a avaliação JSON retorna 2. Nenhuma
captura é publicada e não há próxima requisição. 4xx/5xx esperado corretamente
permite capturar; sem expectativa, não se presume validação de status.

Capturas têm escopo separado das variáveis iniciais e uma sessão nova por execução.
Não há sobrescrita: duplicações e conflitos ancestral/descendente com qualquer
valor inicial, captura anterior ou outra declaração da resposta falham,
independentemente do tipo. O lote só fica disponível se todas as consultas,
valores e nomes passarem. Colisão, captura inválida ou uso antecipado/ausente
retorna 2 e interrompe as entradas seguintes. Resultados anteriores são preservados.
Falha de captura mantém a resposta em `concluídas` e o status aprovado em
`validadas` (ou `sem_validação` sem expectativa/com `HTTP *`), sem classificá-la como transporte.

`--secret nome` pode marcar a captura antes de ela existir, sem fornecer valor.
Um valor secreto descoberto, de qualquer tipo, é protegido antes de apresentar
o resultado, inclusive se outra declaração da resposta falhar. Constituintes de
arrays/objetos também são protegidos nas formas conhecidas, sem virarem variáveis
nem alterar dados enviados. A proteção dinâmica é local à
execução, combina-se com a proteção inicial e cobre variantes JSON/Go/URL
conhecidas e nomes de arquivo em diagnósticos. Vazio não causa substituição global.
Bodies e valores capturados nunca entram no relatório/diagnóstico; valores
selecionados para assertions podem aparecer somente sanitizados conforme abaixo. Nada permite
identificar universalmente segredos ainda desconhecidos ou recodificações arbitrárias.

## Assertions de headers — US-07 / issue #18

**Ampliação intencional do contrato:** `[Asserts]`, antes rejeitada globalmente,
passou a aceitar as três formas de header abaixo; exige `HTTP <status>` ou `HTTP *` na mesma
entrada. A seção não pode ser órfã, vazia ou duplicada. Pode aparecer antes/depois
de `[Captures]`; a ordem de avaliação é status → assertions (headers/JSON na
ordem escrita) → capturas.

```hurl
GET http://localhost:8080/health
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
header "X-Request-Id" exists
header "X-Request-Id" == "req-123"
```

Execute `httpgo test health.hurl` contra uma API que devolva
`Content-Type: application/json; charset=utf-8` e `X-Request-Id: req-123`.
O exemplo está em `testdata/scenarios/scenario_assert_headers.hurl` e é exercitado
pelo binário nos E2E. Trocar `HTTP 200` por `HTTP *` dispensa somente a comparação
de status; assertions e capturas continuam obrigatórias quando declaradas.

- Nomes de header são tokens HTTP estáticos entre aspas duplas, consultados sem
  distinguir caixa. `==` compara a string exata; `contains` procura substring
  literal; valores distinguem caixa e não recebem normalização adicional além
  daquela feita pelo transporte HTTP (por exemplo whitespace externo do protocolo).
- Valores esperados são strings literais entre aspas duplas. `{{nome}}` é texto
  literal, com ou sem variável/captura definida; não passa pelo renderer.
  `exists` não aceita valor esperado. Não há regex, filtros, captura de headers
  ou outros operadores de header. Construções fora desse recorte falham
  explicitamente com posição, antes de qualquer envio do arquivo.
- Havendo ocorrências repetidas, somente a **primeira na ordem recebida** participa
  de `==`/`contains`, inclusive se nomes chegam com caixas diferentes. Nunca há
  concatenação ou divisão por vírgulas: `first,second` é uma única string.
- Ausência faz os três operadores falharem. Presente vazio satisfaz `exists` e
  `== ""`; `contains ""` passa para qualquer valor presente, inclusive vazio.
- A primeira falha interrompe a sequência com código 1, não avalia nem publica
  capturas dessa resposta e mantém os resultados anteriores. Um status numérico
  divergente impede ambas as etapas. Falha de transporte, consumo/fechamento do
  body ou escrita continua retornando 2, nunca aprovação parcial pelos headers.
- O diagnóstico localiza a declaração, identifica header/operador e esperado/
  recebido. `ausente` difere de `""`. Strings comuns usam quoting seguro; valores
  de `Authorization`, `Proxy-Authorization`, `Cookie` e `Set-Cookie` são ocultos
  integralmente nos dois lados, independentemente da caixa ou origem literal.
  Segredos conhecidos também são removidos dos headers comuns antes do callback
  e da escrita. Nenhuma coleção de headers ou body bruto fica no relatório.

Compatibilidade comprovada é limitada aos pares `headers_*` em `compatibility.md`.
**Diferenças deliberadas em relação ao Hurl 8.0.0:** primeira ocorrência em vez de
lista em headers repetidos; esperado literal em vez de interpolado; assertions
antes de capturas. Não se anuncia equivalência nessas combinações.

## Igualdade JSON — US-08 / issue #20

**Ampliação intencional:** `[Asserts]` passa a aceitar também
`jsonpath "consulta" == <literal JSON>`, na mesma lista ordenada das assertions de
headers. Exige `HTTP <status>` ou `HTTP *`; não muda o contrato das capturas.

```hurl
GET {{base_url}}/products/1
HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
jsonpath "$.price" == 4500
jsonpath "$.active" == true
```

Salve como `product.hurl` e execute
`httpgo test product.hurl --var base_url=http://localhost:8080` contra uma API que
retorne `{"name":"Pizza","price":4500,"active":true}`. Exemplo versionado:
`testdata/scenarios/scenario_assert_json.hurl`; fluxo autenticado com headers e ID
`9007199254740993` em `scenario_assert_json_auth_products.hurl`, ambos nos E2E.

- Consulta estática usa exatamente o subconjunto de capturas: `$`, propriedades
  por ponto e índices não negativos combináveis (`$.items[0].name`). Sem
  interpolação de consulta, wildcard, filtros, descida recursiva ou expansão.
- Esperado é **um único literal JSON completo na mesma linha**: string, número,
  booleano, null, array ou objeto, inclusive vazios. Escapes/Unicode seguem JSON.
  Nunca passa pelo renderer: `"{{nome}}"` e placeholders aninhados em strings
  permanecem texto com a variável definida ou ausente. Placeholder sem aspas
  não é literal JSON. Literal inválido/adicional, query/operador fora do recorte
  ou seção sem HTTP em qualquer entrada retorna 2 **antes de qualquer envio**.
- Igualdade preserva tipos, sem coerção: `"4500"` difere de `4500`; `false` difere
  de `0`; `[]` difere de `{}`. Números usam valor decimal exato: `1`, `1.0` e `1e0`
  são iguais; grafias de zero também. Inteiros/decimais grandes e números
  aninhados não arredondam; expoentes não provocam expansão de potências em
  memória. Arrays exigem mesma ordem/comprimento; objetos comparam chaves e
  conteúdo recursivamente, independentemente da ordem das chaves.
- Valor/tipo diferente, propriedade ausente ou índice fora do array é divergência
  (**1**). `null` presente satisfaz `null`; ausência tem representação própria.
  JSON vazio/malformado/com valores adicionais ou percurso por tipo incompatível
  (`$.price.value` com price numérico, propriedade em array ou índice em objeto)
  gera **erro de avaliação JSON (2)**, não de captura/transporte. Capturas
  continuam classificando ausência e percurso incompatível como erro de captura.
- Ordem: resposta consumida/fechada → status numérico, se declarado → assertions
  de headers e JSON na ordem escrita → avaliar/publicar capturas → callback.
  JSON é decodificado somente ao chegar à primeira assertion JSON, reutilizado
  na resposta e descartado depois. A primeira falha interrompe assertions,
  capturas e próximos envios, sem lote publicado. `HTTP *` dispensa só comparação
  numérica; contagens e prioridade de erro de escrita (2) permanecem iguais.
- Diagnóstico localiza a assertion (arquivo/linha/coluna), identifica consulta e
  apresenta esperado/real como JSON sanitizado: string com aspas, número, booleano,
  null ou estrutura. `ausente` difere de `null` e `""`. Nunca despeja o body,
  documento inteiro não selecionado ou coleção de headers. O resultado/callback
  não guarda body, documento decodificado ou literal esperado bruto.
- Segredos marcados, capturas anteriores secretas e derivados/escapes conhecidos
  são removidos de consulta, posição e operandos antes da coleção/formatação.
  Se houver **captura secreta pendente na mesma resposta**, esperado e real de
  falhas de headers **e** JSON ficam integralmente ocultos, inclusive nos metadados
  do callback. A marcação é consultada sem avaliar/resolver/publicar a captura;
  até uma consulta de captura que falharia não altera a primeira falha. A ausência
  continua identificável junto à ocultação. Essa política não reconhece segredos
  arbitrários não marcados e não substitui os limites de segurança documentados.

Continuam fora do recorte: `count`, `exists` JSON, comparações ordenadas, regex,
filtros e interpolação de consultas ou esperados. A gramática de literais
compostos/expoentes difere dos pares observados com Hurl 8.0.0; veja
`compatibility.md`. A semântica aprovada não é alterada para forçar equivalência.

## Execução HTTP

- Cada requisição é iniciada em ordem. Não há paralelismo ou retry, inclusive retry
  implícito do transporte. HTTP/1.1 sem conexão persistente evita o replay de
  requisições em conexão reutilizada. HTTP/2 não é habilitado nesta história.
- Timeout total **fixo de 10 segundos** por requisição, contado uma vez e compartilhado
  por conexão/TLS, redirects e consumo de todos os bodies. Não há flag de ajuste.
- TLS é verificado; não há opção para ignorar certificados.
- Redirects 301/302/303/307/308 com Location são seguidos até **10 saltos**.
  O 11º salto é bloqueado antes de enviar ao próximo destino. Resposta sem
  Location não causa redirect e seu status é o resultado.
- 301/302/303 convertem POST/PUT/PATCH/DELETE em GET **sem body**; GET continua GET.
  307/308 preservam método e bytes do body. Headers declarados são mantidos em
  redirects permitidos. O reenvio de um body em 307/308 não é retry após falha.
- Apenas a mesma origem da requisição inicial: scheme e hostname sem distinção de caixa,
  porta efetiva numérica (80 para HTTP e 443 para HTTPS, quando omitida).
  Não se usa DNS para equiparar hosts: localhost não equivale a 127.0.0.1.
  Mudar scheme/host/porta é erro de transporte, sem contato ao destino proibido.
- Bodies intermediários e respostas sem capturas/assertions JSON são consumidos
  em streaming, sem retenção. Somente o body final de entrada com capturas ou
  assertions JSON é lido em memória, sob o mesmo prazo, e fechado antes de avaliar
  o status final. Headers isolados continuam sem retenção. Bytes e documento
  decodificado não ficam acumulados nos resultados; memória adicional acompanha
  o body final e suas estruturas.
  Resposta incompleta, erro de leitura/fechamento ou timeout no body não aprova
  a expectativa só porque os headers já chegaram. Não há body em relatório.
- Sem expectativa, inclusive 4xx/5xx é sucesso **sem validação de status**.
  Com expectativa numérica, compara-se igualdade com o status final recebido.
  `HTTP *` aceita esse status sem comparação numérica, mas não ignora assertions/capturas.
- Divergência, falha de captura/renderização/transporte ou cancelamento interrompe as próximas entradas.
  Ctrl+C cancela a requisição em andamento. Evidência de sinal real: Linux;
  macOS/Windows ainda não têm aceite nativo (ver `validation.md`).
- Não há cookie jar, proxy configurável, configuração YAML ou leitura automática
  de HTTPGO_*. O transporte não lê HTTP_PROXY/HTTPS_PROXY nesta história.

## CLI e saída

```text
httpgo test <arquivo> [--render-files] [--var nome=valor] [--secret nome]
httpgo --help
httpgo test --help
httpgo --version
```

`test` exige exatamente um arquivo regular legível. Não aceita diretórios,
múltiplos arquivos, entrada de cenário/segredos por stdin nem outras flags.
`--output`, `--output-file`, `check` e configurações futuras
não estão implementados ou anunciados como disponíveis.

Cada entrada iniciada (ou que falha na renderização pré-envio) tem uma linha de
resultado com o método declarado, URL segura e uma
classificação: `status validado`, `sem validação de status`, `status aceito por HTTP *`,
`status divergente`, `assertion de header divergente`, `assertion JSON divergente`,
`erro de avaliação JSON` ou `erro de transporte`; uma entrada que falha antes do envio recebe
`erro de renderização`. Em sucesso, inclusive sem expectativa, a linha começa
diretamente com o método, sem prefixo de arquivo/linha/coluna. Apenas divergências
e erros de transporte/renderização/captura/headers/JSON incluem esse prefixo.
A falha de header/JSON aponta à declaração da assertion. Divergências mostram esperado e
recebido. Falha de transporte não inventa status. `erro de captura` mostra o status
real concluído e se ele foi validado; o diagnóstico em stderr localiza a declaração/consulta.

```text
GET http://localhost:8080/health: status validado (esperado 200, recebido 200)
GET http://localhost:8080/error: sem validação de status (recebido 500)
"health.hurl":1:1: GET http://localhost:8080/health: status divergente (esperado 200, recebido 503)
```

O resumo continua identificando o arquivo, sem linha/coluna.

A linha de cada resultado é escrita **assim que a requisição termina, antes de
iniciar a próxima**, sem esperar o arquivo inteiro terminar. O parsing completo
continua ocorrendo antes de qualquer rede ou resultado. Uma resposta só está
concluída depois de consumir e fechar o body; receber headers não basta para
exibir validação aprovada. As requisições continuam sequenciais, sem goroutines
adicionais para apresentar os resultados. Os códigos de saída são preservados.
A apresentação incremental não reimprime resultados no final.

O resumo é escrito uma única vez ao final da execução, inclusive quando uma
requisição interrompe a sequência por divergência, captura, renderização, transporte ou cancelamento.
O resumo em stdout informa:

- `total`: entradas validadas pelo parser;
- `iniciadas`: chamadas ao executor — **não comprova recebimento pelo servidor**;
- `concluídas`: respostas totalmente consumidas/fechadas, inclusive divergentes;
- `validadas`: comparações de status aprovadas;
- `sem_validação`: respostas concluídas sem comparação numérica (sem expectativa ou `HTTP *`);
  a linha individual distingue esses dois casos. Não significa que headers/JSON não foram avaliados;
- `não_iniciadas`: entradas não iniciadas por interrupção.

Erros de entrada anteriores ao runner não apresentam resumo de execução.
Diagnósticos ficam em stderr e incluem posição quando identificável. Erros
brutos da rede, Location, linha de entrada e bodies nunca são impressos.

| Exit code | Significado |
|---|---|
| 0 | todas as entradas concluídas sem divergência; inclui sem expectativa |
| 1 | primeira divergência de status ou assertion de header/JSON |
| 2 | uso, arquivo, parsing, avaliação JSON, captura, renderização, transporte, TLS, timeout, cancelamento ou escrita |

Erro ao escrever stdout/stderr também é categoria 2; não transforma divergência
em sucesso. Falha na escrita de um resultado interrompe o arquivo antes da requisição
seguinte, preserva o que já foi escrito e não tenta emitir resumo no stdout que
falhou. O erro é diagnosticado de forma segura em stderr, sem ecoar a causa bruta.
A apresentação usa sempre texto simples sem cores/animações,
independentemente de TTY e NO_COLOR: não há detecção de terminal nem formato
alternativo dependente do ambiente. O E2E comprova pipe e PTY Linux real.

`--version` mostra versão, commit e data. Para defini-los no build:

```sh
go build -ldflags '-X main.version=0.1.0 -X main.commit=abc123 -X main.date=2026-11-14' -o bin/httpgo ./cmd/httpgo
```

Sem esses valores, mostra `dev`, `unknown`, `unknown`.

## Segurança e limites

A URL exibível remove userinfo (usuário e senha), **oculta toda a query**, inclusive
seus nomes, e não exibe fragmento. A URL operacional não é mutilada: credenciais e
query continuam disponíveis para a requisição. URL inválida recebe diagnóstico
controlado sem expor o token malformado. O caminho do arquivo é exibido com
quoting para escapar caracteres de controle.

Valores de headers comuns podem aparecer **somente** no diagnóstico de assertion,
com quoting e sanitização. Valores de `Authorization`, `Proxy-Authorization`,
`Cookie` e `Set-Cookie` nunca são apresentados, independentemente da caixa do nome,
nem mesmo quando esperados literais. Bodies de requisição/resposta não são impressos. JSON/header malformado
recebe diagnóstico controlado com arquivo/linha/coluna, sem ecoar entrada ou erro
bruto do decoder/transporte. Isso evita vazamento também de credenciais em bodies.

Valores marcados com `--secret` são removidos da representação exibível **antes**
de normalizações/escapes de URL, inclusive no path. Valores operacionais seguem
intactos para o executor; a representação protegida é separada. Essa proteção
vale também em sucesso, divergência, erros de uso/parsing/renderização/transporte
e falhas de escrita. Declarações são consolidadas independentemente da ordem das
flags. Formas brutas, strings JSON decodificadas e variantes conhecidas de escape
JSON/Go/URL são protegidas também antes da construção do contexto. Constituintes
de objetos/arrays secretos são protegidos sem disponibilizá-los como variáveis.
Valores vazios não provocam substituição global. Coleções de headers e bodies
permanecem ausentes das saídas; não é gerado body de diagnóstico. Codificações arbitrárias desconhecidas
não têm garantia de detecção.

**O restante do path permanece visível: não coloque segredos arbitrários nele.** Não é possível
detectar qualquer segredo arbitrário embutido no path/hostname/nome de arquivo,
nem proteger o conteúdo do arquivo em disco ou argumentos no histórico/processos.
Esta sanitização não é um mecanismo de entrada segura de segredos por flags.

## Diferenças em relação ao Hurl

- O subconjunto é menor: comentários inline, respostas com versão, body em GET,
  headers de enquadramento e demais métodos/seções são rejeitados, não ignorados.
- Hurl 8.0.0 infere `Content-Type: application/json` para JSON inline; httpgo só
  envia esse header se for declarado. Valores e delimitação JSON são compartilhados
  no corpus, não uma promessa de equivalência de todos os headers implícitos.
- Códigos de saída, resultados/resumo e mensagens pertencem ao contrato httpgo,
  não ao Hurl. Ex.: divergência é 1 aqui e 4 no Hurl 8.0.0 usado no corpus.
- Redirects são automáticos mas limitados à mesma origem; o diferencial usa
  `--location --max-redirs 10 --max-time 10 --retry 0` no Hurl para aproximar as
  políticas compartilhadas. Essa aproximação não torna os motores equivalentes.
- Variáveis CLI tipadas são extensão HTTPGo, sem recursão. Não presuma as mesmas
  regras na CLI Hurl: o corpus diferencia strings explicitamente delimitadas,
  estruturas e conversão textual. Interpolação escalar contextual em JSON possui
  pares compartilhados observados, sem anunciar equivalência geral.
- Nomes pontuados são aceitos aqui, mas `api.baseUrl` falhou no Hurl 8.0.0.
  Duplicações/conflitos são erros aqui; Hurl permite sobrescrita no corpus.
  Hurl aceita templates em nomes de headers/chaves JSON; esta entrega não.
- Capturas aqui aceitam todos os tipos JSON desde a US-06, sem sobrescrita.
  Hurl 8.0.0 compartilha número de alta precisão, booleano e null nos pares
  escalares observados, mas os pares de estruturas terminam nele com código 3
  após `/capture`; aqui estruturas completas são preservadas. Não se presume
  equivalência geral de tipos ou conversão textual. Hurl sobrescreve variáveis
  no corpus, não resolve os nomes pontuados dos exemplos e rejeita seção sem
  expectativa. Em valor JSON completo pode reinterpretar texto capturado
  (`"null"` vira null); aqui permanece string. Veja `captures_*`, inclusive
  URI, headers e bytes JSON efetivamente recebidos pela API.
- Headers repetidos são listas no Hurl 8.0.0; aqui usa-se somente a primeira
  ocorrência. Com uma ocorrência, caixa/ausência/vazio/vírgula e os operadores
  têm equivalência observada. Hurl interpola o esperado e avalia capturas antes
  das assertions; aqui o esperado é literal e headers/JSON antecedem capturas.
- Igualdade JSON aqui aceita literais esperados compostos e expoentes exatos;
  Hurl 8.0.0 rejeitou array/objeto e `125e-2` nos pares observados. Tipos escalares,
  precisão inteira e decimais têm pares compartilhados, sem equivalência geral.
  Avaliação inválida/percurso incompatível retorna 2 aqui contra 4 no Hurl;
  ausência de propriedade/índice retorna 1 aqui contra 4. Veja `assert_json_*`.
- `--render-files` é uma **extensão HTTPGo** implementada na US-10, não parte do
  Hurl. Seus testes ficam separados do corpus diferencial em modo bytes, que não
  recebe essa flag. Configuração YAML hierárquica permanece futura e fora do suporte.

Veja a matriz e evidências limitadas ao corpus em `compatibility.md`.
