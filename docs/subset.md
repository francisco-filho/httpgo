# Subconjunto suportado — US-01 a US-05 (issues #4, #6, #11, #13 e #14)

Este documento define somente o que o binário implementa nesta história.
Não implica suporte ao formato Hurl completo.

## Gramática

Um arquivo UTF-8 contém ao menos uma request-line
`<método> <URL absoluta HTTP/HTTPS>`. Métodos aceitos (em maiúsculas):
`GET`, `POST`, `PUT`, `PATCH` e `DELETE`, combináveis em sequência.
A expectativa opcional `HTTP <status>` associa-se à requisição anterior, uma única vez.
O status tem exatamente três dígitos ASCII, entre 100 e 599.
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

`POST`, `PUT`, `PATCH` e `DELETE` aceitam **nenhum body ou um único valor JSON**:
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
templates em chaves JSON e nomes de headers, `[Asserts]` e seções além de
`[Captures]`, `file,...;`, multipart/form-urlencoded, bodies binários/string multilinha
não JSON, versões como `HTTP/1.1` na expectativa ou comentários após instruções.
O texto bruto não é ecoado. O corpus negativo está em
`testdata/scenarios/unsupported_*.hurl` e `invalid_*.hurl`.

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
YAML nesta entrega. Capturas string de respostas não são uma nova fonte de
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

## Capturas string — US-05

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
encerra a seção. Outras queries, filtros e seções não são aceitos nem ignorados.
Nomes seguem a mesma regra ASCII pontuada das variáveis.

O subconjunto de JSONPath aceita `$`, propriedades por ponto e índices decimais
não negativos, combináveis: `$.accessToken`, `$.product.id`, `$.items[0].token` e
`$[0]`. Propriedades simples usam letras ASCII, dígitos e `_`; nomes com outros
caracteres exigiriam uma sintaxe ainda não suportada. Não há wildcard, filtros,
descida recursiva, slices, união, propriedades entre colchetes ou índices negativos.
Consultas inválidas/não suportadas e nomes inválidos em entrada tardia rejeitam
**todo** o arquivo, com arquivo/linha/coluna originais antes de qualquer rede.

A resposta final deve conter exatamente um valor JSON válido e a seleção deve
produzir string, inclusive vazia. Campo ausente/índice fora do array difere de
null, mas ambos falham; números, booleanos, arrays e objetos também são erros de
captura, sem conversão para texto. A string decodificada conserva seu tipo em JSON
completo: `"123"`/`"null"` recebidos não viram número/null. Conteúdo semelhante a
placeholder não é reinterpretado. Escapes contextuais e validações finais de
URL/header/JSON são os mesmos das variáveis iniciais.

Após consumir e fechar a resposta, o status esperado é validado **antes** de
avaliar capturas. Divergência retorna 1 mesmo se a resposta não for JSON; nenhuma
captura é publicada e não há próxima requisição. 4xx/5xx esperado corretamente
permite capturar; sem expectativa, não se presume validação de status.

Capturas têm escopo separado das variáveis iniciais e uma sessão nova por execução.
Não há sobrescrita: duplicações e conflitos ancestral/descendente com qualquer
valor inicial, captura anterior ou outra declaração da resposta falham,
independentemente do tipo inicial. O lote só fica disponível se todas as consultas,
tipos e nomes passarem. Colisão, consulta/tipo inválido ou uso antecipado/ausente
retorna 2 e interrompe as entradas seguintes. Resultados anteriores são preservados.
Falha de captura mantém a resposta em `concluídas` e o status aprovado em
`validadas` (ou `sem_validação` sem expectativa), sem classificá-la como transporte.

`--secret nome` pode marcar a captura antes de ela existir, sem fornecer valor.
Uma string secreta descoberta é protegida antes de apresentar o resultado,
inclusive se outra declaração da resposta falhar. A proteção dinâmica é local à
execução, combina-se com a proteção inicial e cobre variantes JSON/Go/URL
conhecidas e nomes de arquivo em diagnósticos. Vazio não causa substituição global.
Bodies e valores selecionados nunca entram no relatório/diagnóstico. Nada permite
identificar universalmente segredos ainda desconhecidos ou recodificações arbitrárias.

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
- Bodies intermediários e respostas sem capturas são consumidos em streaming,
  sem retenção. Somente o body final de entrada com capturas é lido em memória,
  sob o mesmo prazo, e fechado antes de avaliar o status final. Os bytes são
  usados na captura e não ficam acumulados nos resultados.
  Resposta incompleta, erro de leitura/fechamento ou timeout no body não aprova
  a expectativa só porque os headers já chegaram. Não há body em relatório.
- Sem expectativa, inclusive 4xx/5xx é sucesso **sem validação de status**.
  Com expectativa, a comparação é de igualdade com o status final recebido.
- Divergência, falha de captura/renderização/transporte ou cancelamento interrompe as próximas entradas.
  Ctrl+C cancela a requisição em andamento. Evidência de sinal real: Linux;
  macOS/Windows ainda não têm aceite nativo (ver `validation.md`).
- Não há cookie jar, proxy configurável, configuração YAML ou leitura automática
  de HTTPGO_*. O transporte não lê HTTP_PROXY/HTTPS_PROXY nesta história.

## CLI e saída

```text
httpgo test <arquivo> [--var nome=valor] [--secret nome]
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
classificação: `status validado`, `sem validação de status`, `status divergente`
ou `erro de transporte`; uma entrada que falha antes do envio recebe
`erro de renderização`. Em sucesso, inclusive sem expectativa, a linha começa
diretamente com o método, sem prefixo de arquivo/linha/coluna. Apenas divergências
e erros de transporte/renderização/captura incluem esse prefixo. Divergências mostram esperado e
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
- `sem_validação`: respostas concluídas sem expectativa;
- `não_iniciadas`: entradas não iniciadas por interrupção.

Erros de entrada anteriores ao runner não apresentam resumo de execução.
Diagnósticos ficam em stderr e incluem posição quando identificável. Erros
brutos da rede, Location, linha de entrada e bodies nunca são impressos.

| Exit code | Significado |
|---|---|
| 0 | todas as entradas concluídas sem divergência; inclui sem expectativa |
| 1 | primeira divergência de status |
| 2 | uso, arquivo, parsing, captura, renderização, transporte, TLS, timeout, cancelamento ou escrita |

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

Nenhum valor de header é apresentado, incluindo `Authorization`,
`Proxy-Authorization`, `Cookie` e `Set-Cookie`, independentemente da caixa do nome.
Bodies de requisição/resposta também não são impressos. JSON/header malformado
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
Valores vazios não provocam substituição global. Headers/bodies permanecem ausentes
das saídas; não é gerado body de diagnóstico. Codificações arbitrárias desconhecidas
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
- Capturas aqui são somente strings, sem sobrescrita. Hurl 8.0.0 aceita outros
  tipos e sobrescreve variáveis no corpus. Nomes pontuados não resolvem nele;
  seção sem expectativa é rejeitada. Em valor JSON completo Hurl pode reinterpretar
  o texto capturado (`"null"` vira null); aqui permanece string. Veja os pares
  `captures_*`, que verificam inclusive Authorization recebido pela API.
- Configuração YAML hierárquica e interpolação externa, quando implementadas,
  serão **extensões httpgo**, não parte do Hurl; ambas continuam fora desta história.

Veja a matriz e evidências limitadas ao corpus em `compatibility.md`.
