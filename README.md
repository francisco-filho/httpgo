# HTTPGo

HTTPGo é uma CLI independente em Go para testar APIs HTTP usando arquivos `.httpgo`.
É inspirada no [Hurl](https://hurl.dev/), mas implementada sem reutilizar seu
código-fonte. A compatibilidade é **básica e restrita ao subconjunto documentado**:
não há promessa de suportar toda a linguagem nem de equivalência entre as CLIs.

## Por onde começar

1. [Instalação](#instalação)
2. [Primeira execução e resultado](#primeira-execução-e-resultado)
3. [Requisições e dados de entrada](#requisições-e-dados-de-entrada)
4. [Variáveis e encadeamento](#variáveis-e-encadeamento)
5. [Assertions e captures](#assertions-e-captures)
6. [Configuração, suítes e relatórios](#configuração-suítes-e-relatórios)
7. [Extensões e limites de compatibilidade](#extensões-e-limites-de-compatibilidade)
8. [Receitas adicionais](#receitas-adicionais) e [desenvolvimento](#documentação-e-desenvolvimento)

## Instalação

A instalação descrita aqui é **a partir do código-fonte**; não pressupõe release
publicada, gerenciador de pacotes ou script remoto de instalação. Tenha Git e
**Go 1.26.10**, conforme [go.mod](go.mod). O executável é independente, sem Go ou
outro runtime na máquina em que será usado.

```sh
git clone https://github.com/francisco-filho/httpgo.git
cd httpgo
go build -trimpath -o bin/httpgo ./cmd/httpgo
```

No Windows, use `go build -trimpath -o bin/httpgo.exe ./cmd/httpgo`. Adicione o
diretório `bin` ao `PATH` ou substitua `httpgo` nos comandos abaixo por
`./bin/httpgo` (Windows: `.\bin\httpgo.exe`). Se já clonou o projeto, basta compilar
na raiz. Não é necessário instalar Hurl para usar HTTPGo.

```sh
httpgo --version
httpgo --help
httpgo test --help
```

Sem metadados de release, a versão desta compilação é
`httpgo dev commit=unknown date=unknown`; isso não é uma versão publicada.
A [referência do subconjunto](docs/subset.md) define as capacidades desta revisão.

## Primeira execução e resultado

Vamos usar uma API local fornecida pelo próprio repositório, sem depender de um
serviço público. Na raiz do clone, em um primeiro terminal:

```sh
go run ./tools/dummy/main.go
```

Mantenha-o aberto: o servidor escuta apenas em `127.0.0.1:8080`, cuja porta precisa
estar livre. `GET /health` responde **200**, `Content-Type: application/json` e
`{"status":"ok"}`. Encerre com **Ctrl+C** ao terminar o guia. Ele é uma fixture
manual, sem persistência, TLS ou autenticação; nunca envie credenciais reais.
Veja os [endpoints e limites do servidor](docs/dev-server.md).

Em outro terminal, crie uma pasta de trabalho vazia **fora do clone**, entre nela
e salve o arquivo `exemplo.httpgo` com o conteúdo completo:

```hurl
GET {{base_url}}/health
HTTP 200
```

A primeira linha define método e URL; `HTTP 200` exige o status da resposta.
`{{base_url}}` é uma variável: `--var nome=valor` fornece seu valor não sigiloso.
Os exemplos seguintes usam essa mesma pasta e o mesmo servidor, salvo indicação.
Os comandos de shell do guia usam sintaxe POSIX (sh/bash); adapte caminhos e a
leitura do código de saída ao PowerShell (`$LASTEXITCODE`) ou cmd (`%ERRORLEVEL%`).

```sh
httpgo test exemplo.httpgo --var base_url=http://localhost:8080
echo $?
```

A saída em stdout informa o GET, status 200 e resumo com uma requisição validada;
o código é **0**, sem diagnóstico de erro em stderr. A CLI não imprime o body.
Agora troque **somente** `HTTP 200` por `HTTP 201` no arquivo e repita o comando:
o servidor continua respondendo 200, aparece **status divergente**, e `echo $?`
mostra **1**. Restaure `HTTP 200` antes de prosseguir.

| Código | Significado |
|---|---|
| `0` | Execução aprovada pelas validações declaradas |
| `1` | Falha de assertion, incluindo status divergente |
| `2` | Demais erros: uso, sintaxe, configuração, captura, avaliação, renderização, transporte, cancelamento ou escrita de relatório |

Em suítes, **2 prevalece sobre 1**. Sem uma linha `HTTP`, qualquer status é aceito
e informado como **sem validação de status**; `HTTP *` também aceita qualquer
status, não equivale a exigir 200. Dentro de um arquivo, uma falha definitiva
interrompe as próximas requisições; não desfaz efeitos que já ocorreram na API.
A saída é texto sem cores, inclusive em TTY e com `NO_COLOR`; resultados vão a
stdout e diagnósticos a stderr. Consulte os [contratos da CLI](docs/subset.md#cli-e-saída).

## Requisições e dados de entrada

### Headers e body JSON

Após o GET inicial, envie dados com POST. O servidor local aceita `POST /products`
e responde **201** com `{"id":1}`. Headers ficam após a linha da requisição;
o body JSON vem depois deles. Salve `json.httpgo`:

```hurl
POST http://localhost:8080/products
Content-Type: application/json
X-Client: guia-httpgo
{"name":"Pizza","price":4500}
HTTP 201
```

```sh
httpgo test json.httpgo
```

Resultado: código **0**, um POST com status 201 validado. Também há PUT, PATCH e
DELETE; GET não aceita body. Cada arquivo pode conter várias requisições
sequenciais. Declare `Content-Type` explicitamente: HTTPGo não infere
`application/json` para o body completo. Veja [métodos, headers e JSON inline](docs/subset.md#headers-e-json-inline).

### Enviar um arquivo como body completo

Crie as pastas `tests` e `payloads` na pasta de trabalho. Salve
`payloads/product.json` com `{"name":"Pizza","price":4500}` e
`tests/arquivo.httpgo` com:

```hurl
POST http://localhost:8080/products
Content-Type: application/json
file,../payloads/product.json;
HTTP 201
```

```sh
httpgo test tests/arquivo.httpgo
```

O dummy recebe os bytes do JSON e retorna 201: código **0**. `file,...;` substitui
o body inteiro. O caminho é literal e relativo ao **arquivo de cenário**, não ao
terminal; não aceita interpolação. Sem `--render-files`, os bytes são preservados,
inclusive `{{placeholder}}`, arquivos vazios e binários. Para enviar um binário,
use uma API que o aceite e declare seu Content-Type e status esperado.
**Não há sandbox**: caminhos absolutos e `..` podem ler fora do projeto. Execute
somente cenários confiáveis. Veja [arquivos externos](docs/subset.md#body-de-arquivo-externo).

### Enviar multipart

Multipart envia campos textuais e **um arquivo**, em POST, PUT ou PATCH. Neste
exemplo é necessária **outra API**, em `http://localhost:8081`, com
`POST /documents` aceitando os campos `description`, `category` e o arquivo
`document`, e respondendo **201**. O dummy em 8080 **não aceita multipart**.
Use apenas um servidor de testes que cumpra esse contrato, sem credenciais reais.

Crie a pasta `fixtures`, salve `fixtures/document.txt` com o texto
`Documento fictício` e salve `tests/upload.httpgo`:

```hurl
POST http://localhost:8081/documents
[MultipartFormData]
description: Documento de teste
category: reports
document: file,../fixtures/document.txt; text/plain
HTTP 201
```

```sh
httpgo test tests/upload.httpgo
```

Com a API indicada, o resultado é **0**, um POST validado; a API recebe os dois
textos e o arquivo com filename `document.txt`. Boundary e Content-Type são
automáticos: não forneça boundary manual. Só campos textuais interpolam; os bytes
do arquivo nunca são renderizados, mesmo com `--render-files`. Não combine outro
body na mesma entrada. Múltiplos arquivos e campos repetidos não são suportados.
Veja [contrato multipart e MIME](docs/subset.md#upload-multipart-básico).

## Variáveis e encadeamento

Use `{{nome}}` em URLs, valores de headers e JSON para reutilizar um cenário.
`--var nome=valor` pode ser repetido para nomes diferentes; forneça somente dados
não sigilosos. Literal JSON preserva tipo, o restante é texto. Nomes pontuados
são exatos: `product.name` não é acesso à propriedade de um objeto `product`.

Salve `products.httpgo`:

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"name":"{{product.name}}","price":{{product.price}},"active":{{product.active}},"code":{{product.code}}}
HTTP 201
```

```sh
httpgo test products.httpgo --var api.baseUrl=http://localhost:8080 \
  --var product.name=Pizza --var product.price=4500 \
  --var product.active=true --var 'product.code="001"'
```

O dummy responde 201: resultado **0**. A API recebe nome e código como strings,
preço como número e `active` como booleano. As aspas internas em
`'product.code="001"'` forçam string. Troque `api.baseUrl` para outro destino sem
editar o cenário. Duplicar um nome na CLI ou misturar `api` e `api.baseUrl` é erro
**2**. A URL não recebe percent-encoding automático.

Em um valor JSON completo, placeholders sem aspas preservam tipo e precisão,
inclusive arrays/objetos; dentro de strings JSON, URLs e headers, só escalares
são aceitos. Strings JSON são escapadas corretamente. Não há expansão recursiva,
navegação de propriedades de objetos CLI ou nomes de headers/chaves JSON
dinâmicos. Consulte [variáveis e templates](docs/subset.md#variáveis-tipadas-e-templates).
Para encadear valores **recebidos** da API, em vez de fornecidos inicialmente,
use captures, explicadas a seguir. Credenciais reais vêm do
[ambiente do processo](docs/configuration.md#referências-ao-ambiente-do-processo-e-segredos), nunca de flags.

## Assertions e captures

**Assertion valida** uma propriedade da resposta; **capture extrai** um valor
para uso posterior. Capturar `id` não prova que o produto tem o nome correto;
exigir `HTTP 200` não prova que o JSON está correto. Combine as verificações
necessárias ao comportamento que você quer testar.

### Validar headers, JSON e texto

O dummy retorna em `GET /products/1` o JSON
`{"id":1,"name":"Pizza","price":4500}` e Content-Type `application/json`.
Salve `validacoes.httpgo`:

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
```

Resultado: **0**, com status e assertions aprovados. `header` consulta o primeiro
valor do header (nome sem distinguir caixa); `jsonpath "$.name"` seleciona um
campo; `body contains` busca substring no texto final decodificado, sem
normalizar JSON, espaços ou caixa. A busca literal de `passwordHash` não prova
a ausência de todas as codificações possíveis de um segredo.

Troque `"Pizza"` por `"Suco"` **só no esperado da assertion JSON**: retorna **1**,
com diagnóstico da consulta e localização da falha; não é erro de transporte.
Restaure o esperado antes de seguir. Tipos importam: `4500` difere de `"4500"`.
Campo ausente não é `null` presente. JSON inválido ou percurso incompatível é erro
de avaliação **2**, não sucesso por negação.

### Capturar um ID e encadear requisições

Salve `encadeamento.httpgo`. O POST do dummy sempre retorna `{"id":1}` com 201;
o GET desse ID retorna o produto acima. São fixtures fixas, não persistência.

```hurl
POST {{base_url}}/products
Content-Type: application/json
{"name":"Pizza","price":4500}
HTTP 201
[Captures]
product_id: jsonpath "$.id"
[Asserts]
jsonpath "$.id" == 1

GET {{base_url}}/products/{{product_id}}
HTTP 200
[Asserts]
jsonpath "$.name" == "Pizza"
```

```sh
httpgo test encadeamento.httpgo --var base_url=http://localhost:8080
```

Resultado: **0**, dois envios aprovados, sendo o segundo `GET /products/1`.
O ID capturado conserva o tipo numérico. Para também comparar o ID retornado
pelo GET com a captura, acrescente `jsonpath "$.id" == {{product_id}}` ao segundo
`[Asserts]` e use:

```sh
httpgo test encadeamento.httpgo --render-asserts --var base_url=http://localhost:8080
```

Com essa alteração, continua **0**. `--render-asserts` é um opt-in HTTPGo:
resolve esperados dinâmicos; sem a flag, strings esperadas com `{{...}}` são
literais, e placeholders fora de literal válido são rejeitados. Um valor JSON
completo sem aspas conserva tipo; `"{{product_id}}"` seria string, diferente de 1.

### O que acontece quando uma etapa falha

A ordem é **resposta completa → status → todas as capturas provisórias →
assertions na ordem escrita → confirmação do lote inteiro**. Assertions podem
usar capturas da própria resposta, mesmo invertendo a ordem das seções, mas uma
consulta de captura não pode depender de outra captura do mesmo lote.

- Status divergente retorna **1** antes das capturas. Assertion divergente
  retorna **1** e descarta o lote provisório. Por exemplo, exigir ID 2 no POST
  acima impede o GET: `product_id` não é publicado.
- Captura ausente/inválida é **2** e precede assertions; avaliação inválida,
  cancelamento ou timeout também impedem confirmação e próximo envio.
- Não há publicação parcial nem rollback de efeitos na API. Capturas confirmadas
  pertencem só ao arquivo; o próximo arquivo recomeça com as variáveis iniciais.
- Captura pode renovar um nome exatamente igual, inclusive inicial, mas nomes
  duplicados no mesmo lote e conflitos como `item`/`item.id` são erro **2**.
- Sem `[Options]` de retry, não há reenvio automático. O
  [polling opt-in](docs/subset.md#polling-local-por-entrada)
  repete a requisição completa só após divergência **1**, podendo repetir efeitos;
  somente a tentativa aprovada confirma capturas. Erro **2** não é retentado.

Para aprofundar, consulte a [gramática](docs/subset.md#gramática),
[assertions de headers](docs/subset.md#assertions-de-headers),
[igualdade JSON](docs/subset.md#igualdade-json),
[presença, quantidade e limites](docs/subset.md#presença-quantidade-e-limites),
[tipos e vazios](docs/subset.md#tipos-e-valores-vazios-json),
[busca textual](docs/subset.md#busca-textual-no-body),
[capturas tipadas](docs/subset.md#capturas-tipadas),
[esperados dinâmicos](docs/subset.md#esperados-dinâmicos) e
[publicação transacional](docs/subset.md#renovação-transacional-de-capturas).
Essas referências contêm a sintaxe completa, consultas, predicados e restrições;
as [receitas adicionais](#receitas-adicionais) mostram combinações opcionais,
sem transformar o guia em catálogo de toda a linguagem.

## Configuração, suítes e relatórios

### Configurar o destino e escolher um ambiente

Reutilize destinos e variáveis com **configuração YAML hierárquica, uma extensão
HTTPGo**. O [guia de configuração](docs/configuration.md) reúne os arquivos
completos, a precedência base → ambiente → CLI, schema, limites e overrides.
Ele demonstra `httpgo test exemplo.httpgo --env local --var base_url=http://localhost:9090`
e distingue ambientes nomeados de referências explícitas `${NOME}` ao processo.
Credenciais reais vêm do ambiente do processo, **nunca de argumentos**; consulte
no guia as fontes disponíveis, proteção preservada após override e seus limites.

### Executar uma suíte e gerar relatórios

Crie uma pasta separada `suite` (não use o clone ou a pasta com todos os exercícios).
Copie `exemplo.httpgo`, restaurado com `HTTP 200`, para `suite/01-health.httpgo`.
Salve `suite/02-produto.httpgo` com:

```hurl
GET {{base_url}}/products/1
HTTP 200
```

```sh
httpgo test suite/ --var base_url=http://localhost:8080
httpgo test suite/ --var base_url=http://localhost:8080 --output junit --output-file target/test-results.xml
httpgo test suite/ --var base_url=http://localhost:8080 --fail-fast --output html --output-file target/report.html
```

Os três comandos retornam **0**: dois arquivos, duas requisições aprovadas.
Diretórios descobrem cenários recursivamente, em ordem lexicográfica; arquivos e
requisições executam sequencialmente. Capturas não passam de um arquivo para outro.
Sem `--fail-fast`, a falha de um arquivo não impede os demais; com a flag, os
restantes ficam **não executados**. Erros de descoberta impedem todo HTTP.

O primeiro comando mostra resultados e resumo em stdout. Nos outros, stdout fica
vazio: JUnit grava `target/test-results.xml`, substituindo destino existente;
HTML gera `target/report-YYYYMMDDTHHMMSSmmmZ.html` em UTC, sem sobrescrever
anteriores (adiciona sufixo na colisão), e anuncia o caminho final em stderr.
Ambos criam diretórios pais quando possível, exigem `--output-file` e reaproveitam
a execução, **sem reenviar requisições para gerar o relatório**.

JUnit tem um testcase por arquivo: divergência vira `failure`, demais erros
viram `error`, não executados viram `skipped`. O HTML é um arquivo autossuficiente
para abrir offline, com resumo e detalhes por requisição. Também são gerados com
falhas ocorridas na execução; erro global anterior à execução não cria relatório.
Erro de escrita/fechamento retorna **2**, podendo deixar arquivo parcial.
Não há JSON, HTML/JUnit simultâneos nem publicação externa automática.
Os relatórios não incluem bodies/headers integrais, mas ainda exigem marcação de
segredos. Consulte [suítes](docs/subset.md#suítes),
[JUnit e CI](docs/junit.md) e [HTML e proteção de dados](docs/html.md).

## Extensões e limites de compatibilidade

### Extensões de arquivo e compatibilidade

`.httpgo` é a extensão padrão recomendada para novos cenários. Arquivos `.hurl`
continuam aceitos, sem renomeação, conversão de conteúdo ou flag adicional.
Ambas usam a mesma linguagem e execução: a extensão não seleciona modo de
compatibilidade nem amplia o subconjunto Hurl suportado. Aceitação da extensão
não significa compatibilidade integral com Hurl.

Para reproduzir uma suíte mista, com o dummy do guia ativo em 8080, crie uma
pasta separada `tests` contendo `01-health.httpgo` e `02-health.hurl`, ambos com:

```hurl
GET http://localhost:8080/health
HTTP 200
```

```sh
httpgo test tests/01-health.httpgo
httpgo test tests/02-health.hurl
httpgo test tests/
```

Cada chamada individual aprova uma requisição e retorna **0**. O diretório
executa ambos nessa ordem e resume dois arquivos aprovados, também com **0**.
A descoberta recursiva inclui `.httpgo` e `.hurl` sem preferência por extensão;
caminhos distintos com conteúdo idêntico continuam cenários distintos. Caminhos
equivalentes e hard links são deduplicados por identidade do filesystem. Outras
extensões não entram na descoberta, mas arquivos explícitos, inclusive sem
extensão, continuam aceitos e validados pelo conteúdo. Não há migração obrigatória.
Veja o [contrato de compatibilidade das extensões](docs/compatibility.md#extensões-de-arquivo).

### Recursos próprios do HTTPGo

**Extensão própria não significa recurso ausente no Hurl por princípio.** Hurl
também tem variáveis, captures, assertions, envio de arquivos, multipart,
configuração e relatórios. O que é próprio aqui são os **contratos concretos**
abaixo, não essas capacidades genéricas:

- **Configuração YAML hierárquica**: `.httpgo.yaml`, `--config`, ambientes em
  `environments/<nome>.yaml`, merge tipado e referências completas `${NOME}`.
  Serve para compartilhar destinos, timeouts e nomes de variáveis sem editar
  cenários. O [guia de configuração](docs/configuration.md) traz base, ambiente,
  cenário, comando e resultado completos. Não é sintaxe de cenário nem um formato
  de configuração intercambiável com Hurl.
- **Renderização opt-in de JSON externo**: `--render-files` aplica templates aos
  arquivos usados como body completo, preservando tipos como no JSON inline.
  Permite reutilizar payloads entre cenários sem prometer que Hurl renderize os
  mesmos arquivos. Sem flag, o envio externo continua sendo bytes literais.

Para experimentar a segunda extensão, salve `payloads/product-template.json` com
este **template** (antes da substituição, não é um JSON válido):

```text
{"name":"{{product.name}}","price":{{product.price}}}
```

Salve `tests/renderizado.httpgo`:

```hurl
POST http://localhost:8080/products
Content-Type: application/json
file,../payloads/product-template.json;
HTTP 201
```

```sh
httpgo test tests/renderizado.httpgo --render-files --var product.name=Pizza --var product.price=4500
```

O dummy recebe `{"name":"Pizza","price":4500}` e retorna 201; resultado **0**.
Sem `--render-files`, o mesmo arquivo é enviado literalmente: neste exemplo o
dummy rejeita o template não resolvido com 400, causando divergência **1**.
Não é a CLI validando JSON externo no modo bytes. Com a flag, variável ausente,
JSON inválido ou tipo incompatível é renderização **2**, antes do envio afetado.
Anteriores não são desfeitas. A flag vale para todos os bodies externos completos
da invocação, não depende da extensão do arquivo e **não renderiza multipart**.
Veja o [contrato de renderização](docs/subset.md#extensão-httpgo-json-externo-com---render-files).

### Separar extensões, diferenças e ausência de suporte

| Classificação | Exemplo e consequência |
|---|---|
| Subconjunto compartilhado | Request-line, `HTTP 200`, `[Asserts]`, `[Captures]` e multipart básico, nos recortes com evidência diferencial |
| Extensão HTTPGo | YAML hierárquico e `--render-files`, testados separadamente; não passe essas opções ao Hurl |
| Diferença de comportamento | Esperados são literais por padrão; `--render-asserts` opta por esperados dinâmicos. Headers repetidos usam a primeira ocorrência; códigos e relatórios têm contrato próprio |
| Não suportado | XPath, query `variable`, JSONPath completo, `body matches`, multipart com múltiplos arquivos, execução paralela e `httpgo check` |

Construções reconhecidas fora do subconjunto não são ignoradas: retornam erro
explícito **2**, com localização, e impedem execução parcial do arquivo por erro
estático. A [referência do subconjunto](docs/subset.md), as
[diferenças em relação ao Hurl](docs/subset.md#diferenças-em-relação-ao-hurl) e a
[matriz de compatibilidade](docs/compatibility.md) são a fonte dos limites.
A matriz fixa **Hurl 8.0.0** e classifica evidências por cenário; aceitar o parsing
não comprova equivalência de execução, transporte, segredos ou CLI.

Para comparar os conceitos com a documentação pública do Hurl, consulte
[templates e fontes de variáveis](https://hurl.dev/docs/templates.html),
[arquivos de configuração](https://hurl.dev/docs/manual.html#configuration) e
[opções da CLI e relatórios](https://hurl.dev/docs/manual.html).
Essas páginas podem evoluir; não substituem a referência versionada da matriz.

## Receitas adicionais

Esta parte é opcional: combina conceitos já apresentados, sem listar todos os
operadores. Os exemplos abaixo têm fixtures versionadas e testes HTTP locais,
mas **não usam o dummy**: cada receita requer uma API de testes com o contrato
indicado. Os contratos são independentes, não rotas a acrescentar à sua API de
produção. Nos comandos, execute na raiz do clone e ajuste `base_url`/`mailbox_url`
para essa API. Não rode toda `testdata/scenarios` como suíte: ela contém casos
intencionalmente inválidos e não suportados.

<details>
<summary>Cache condicional e formato de cookie</summary>

A API de cache responde a `GET /catalog` com 200 e `ETag: "v1"`; com
`If-None-Match: "v1"`, retorna 304. Salve como `cache.httpgo` ou use a fixture:

```hurl
GET {{base_url}}/catalog
HTTP 200
[Captures]
catalog_etag: header "ETag"

GET {{base_url}}/catalog
If-None-Match: {{catalog_etag}}
HTTP 304
```

```sh
httpgo test testdata/scenarios/scenario_capture_cache.httpgo --var base_url=http://localhost:8081
```

Resultado **0**, dois envios; ausência de ETag é captura **2**, sem segundo envio.
Capturar header conserva string, aspas e primeira ocorrência. Não cria cookie jar.
[Referência de captura de header](docs/subset.md#capturas-literais-de-headers).

Para outra API, `GET /session` deve responder 200 e
`Set-Cookie: session=ficticio; Path=/; HttpOnly; Secure; SameSite=Lax`:

```hurl
GET {{base_url}}/session

HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
```

```sh
httpgo test testdata/scenarios/scenario_assert_header_matches.httpgo --var base_url=http://localhost:8081
```

Resultado **0**; retirar `Secure` falha com **1**. Regex usa sintaxe Go, não
lookaround/backreferences. [Regex de headers](docs/subset.md#regex-de-headers).

</details>

<details>
<summary>Extrair código de texto e reutilizar cookie explicitamente</summary>

Requer `GET /message/123` com 200 e `Seu código de verificação é: 123456`, e
`POST /verify` aceitando `{"code":"123456"}` com 200. Extração seleciona o primeiro
grupo da primeira correspondência; marque o código como segredo:

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

```sh
httpgo test testdata/scenarios/scenario_capture_otp.httpgo --var mailbox_url=http://localhost:8081 --var base_url=http://localhost:8081 --secret otp_code
```

Resultado **0**, dois envios sem revelar o código. Sem correspondência, captura
**2**, sem POST. Para também **validar** o texto extraído, use esta variante:

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

Resultado **0**; exigir sete dígitos só no esperado retorna **1**, sem POST e sem
confirmar a captura. [Capturas regex](docs/subset.md#extração-regex-de-body-e-header)
e [assertions regex](docs/subset.md#assertions-sobre-extração-regex).

Em outra API, `GET /session` responde 200 e `Set-Cookie: refresh=ficticio; Path=/`;
`GET /next` aceita `Cookie: refresh=ficticio` e responde 200:

```hurl
GET {{base_url}}/session
HTTP 200
[Captures]
refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"

GET {{base_url}}/next
Cookie: refresh={{refresh_token}}
HTTP 200
```

```sh
httpgo test testdata/scenarios/scenario_capture_refresh.httpgo --var base_url=http://localhost:8081 --secret refresh_token
```

Resultado **0**. Envie só o cookie extraído, não os atributos de `Set-Cookie`.

</details>

<details>
<summary>Inclusão, ausência, tipos e projeção de arrays</summary>

Para inclusão e ausência, `GET /validation` responde 200 com
`{"codes":["REQUIRED","INVALID"],"name":"Pizza"}` e cookie sem `Domain`:

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.codes" contains "REQUIRED"
jsonpath "$.codes" not contains "OTHER_TENANT"
jsonpath "$.passwordHash" not exists
header "Set-Cookie" not contains "Domain"
```

```sh
httpgo test testdata/scenarios/scenario_assert_membership.httpgo --var base_url=http://localhost:8081
```

Resultado **0**; acrescentar `OTHER_TENANT` aos códigos falha com **1**.
[Inclusão e ausência](docs/subset.md#inclusão-e-ausência-jsonheader).

Para tipos, `GET /session-info` responde 200 com
`{"token":"abc","attempts":3,"active":false,"issues":[]}` (token fictício):

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
httpgo test testdata/scenarios/json_types.httpgo --var base_url=http://localhost:8081
```

Resultado **0**; `attempts: 3.0` falha com **1**: `isInteger` exige notação inteira,
ao contrário da igualdade numérica. [Tipos e vazios](docs/subset.md#tipos-e-valores-vazios-json).

Para projetar arrays, **outro contrato** de `GET /validation` retorna
`{"issues":[{"code":"INVALID"},{"code":"REQUIRED"}]}` com 200; `POST /echo` aceita o
array `["INVALID","REQUIRED"]` e retorna **204**:

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

```sh
httpgo test testdata/scenarios/scenario_projection.httpgo --var base_url=http://localhost:8081
```

Resultado **0**, dois envios. `[*]` projeta arrays; zero resultados é ausência,
um conserva seu tipo, vários formam array, sem achatamento. Não serve para
objetos nem implica JSONPath completo. [Seleção e limites](docs/subset.md#igualdade-json).

</details>

<details>
<summary>Selecionar entidades e comparar com valores anteriores</summary>

Para filtro estático, `GET /products` responde 200 com
`[{"slug":"pizza","id":1,"active":true}]`; `GET /products/1` responde 200:

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

```sh
httpgo test testdata/scenarios/scenario_filter.httpgo --var base_url=http://localhost:8081
```

Resultado **0**; sem pizza, captura **2**, sem GET final. O filtro suporta igualdade
escalar, não lógica arbitrária. [Filtros estáticos](docs/subset.md#seleção-estática-por-igualdade).

Para esperados dinâmicos, `POST /orders` e `GET /orders/9007199254740993` respondem
200 com `{"number":9007199254740993,"name":"Pizza"}`:

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

Resultado **0**, preservando precisão. Mudar o número só no GET falha com **1**.

Para filtro dinâmico, `GET /first` responde `{"number":42}` com 200;
`GET /orders` responde `{"items":[{"number":42,"status":"PLACED","id":"selected"}]}`
com 200; `GET /selected` responde 200:

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

```sh
httpgo test testdata/scenarios/dynamic_filter.httpgo --render-asserts --var base_url=http://localhost:8081
```

Resultado **0**, três envios. Operandos são dados, não nova gramática; a flag não
habilita propriedades, operadores ou consultas inteiras dinâmicas.
[Operandos dinâmicos e limites](docs/subset.md#operandos-dinâmicos).

</details>

<details>
<summary>Comparar quantidade e verificar rotação de token</summary>

`GET /orders` retorna 200 com `{"items":[1,2,3]}` antes e depois;
`POST /orders/preview` aceita `{"productId":"p1"}` e retorna 200 sem mudar quantidade:

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

```sh
httpgo test testdata/scenarios/scenario_count_unchanged.httpgo --render-asserts --var base_url=http://localhost:8081
```

Resultado **0**; quatro itens depois falham com **1**. `count` conta o array final,
não conteúdo igual nem tamanho de string. [Captura de contagem](docs/subset.md#captura-de-contagem).

Para rotação, `POST /session/login` responde 200 com
`{"accessToken":"antigo-ficticio"}`; `POST /session/renew`, 200 com
`{"accessToken":"novo-ficticio"}`; `GET /next` aceita o novo Bearer e retorna 200:

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

Resultado **0**; repetição ou ausência do token novo falha com **1**, sem GET.
Isso prova diferença, **não** revogação do antigo nem validade do novo para outras
operações. [Desigualdade JSON](docs/subset.md#desigualdade-json).

</details>

## Documentação e desenvolvimento

- [Subconjunto, sintaxe, execução e segurança](docs/subset.md)
- [Compatibilidade diferencial com Hurl 8.0.0](docs/compatibility.md)
- [Arquitetura e ciclo de vida](docs/architecture.md)
- [Servidor dummy para desenvolvimento](docs/dev-server.md)
- [Relatório JUnit e CI](docs/junit.md) e [HTML offline](docs/html.md)
- [Comandos e evidências de validação](docs/validation.md)
- [Regras do repositório](AGENTS.md) e [targets disponíveis](Makefile)

Para desenvolver, tenha também `goimports` e `golangci-lint` no `PATH`.
Os testes com `-race` exigem CGO e compilador/headers C no ambiente de
**desenvolvimento**, não no binário entregue. Na raiz do clone:

```sh
make fmt lint test
go vet ./...
make build
```

A suíte padrão usa servidores locais `httptest`, não depende do dummy nem de APIs
externas. O diferencial Hurl é um gate separado com versão fixada, descrito na
[matriz e seus pré-requisitos](docs/compatibility.md#escopo-da-evidência).
Build cruzado não comprova execução nativa; esta documentação não anuncia
artefatos de release nem homologação em todas as plataformas.
