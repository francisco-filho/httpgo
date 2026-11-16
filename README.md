# httpgo

CLI independente em Go para executar um **subconjunto documentado** de arquivos
`.hurl`. Implementação própria, sem reutilizar código-fonte do Hurl.

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

Sem paralelismo, dependências entre arquivos, login compartilhado, JUnit,
`--output`/`--output-file` ou novos formatos. O contrato detalhado dos contadores
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
stdin, composição parcial, referências em listas ou JUnit. É **extensão HTTPGo**,
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
Capturas de respostas estão disponíveis desde a US-05, ampliadas para qualquer
tipo JSON na US-06.

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
JSONPath aceita somente raiz `$`, propriedades por ponto (`$.product.id`) e
índices de array não negativos (`$.items[0]`). Desde a US-06, o resultado aceita
qualquer tipo JSON, inclusive raiz, `null`, string vazia, arrays/objetos vazios.
JSON inválido, caminho ausente ou índice fora do array é erro de captura (código 2).
Consultas não suportadas em qualquer entrada impedem todos os envios.

Capturas ficam disponíveis após consumir/fechar a resposta e aprovar o status,
quando esperado, e todas as assertions de headers/JSON declaradas, para URLs, valores de
headers, JSON inline e JSON externo com `--render-files` seguintes. Strings
como `"123"`, `"null"` ou `"{{literal}}"` continuam strings, sem reinterpretação.
Nomes pontuados resolvem por nome exato; duplicações e conflitos hierárquicos com
variáveis iniciais ou capturas anteriores falham sem sobrescrever. Cada arquivo
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
sequenciais e a primeira falha de status, header, JSON, captura, renderização ou transporte
interrompe as próximas requisições do mesmo arquivo; outros arquivos seguem
conforme `--fail-fast`. Uma falha de captura preserva a resposta concluída e o
status aprovado nas contagens, mas impede o próximo envio. Sem `HTTP`, qualquer status recebido é aceito,
com indicação explícita de **sem validação de status**.

- `0`: sucesso; `1`: status ou assertion de header/JSON divergente; `2`: demais erros,
  inclusive avaliação de JSON inválido ou percurso incompatível.
- Timeout total configurável em YAML, default de 10 segundos por requisição,
  incluindo redirects e leitura do body.
- TLS verificado; até 10 redirects, apenas na mesma origem; nenhum retry.
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
`{{nome}}` no esperado permanece literal, sem interpolação. Regex, filtros,
captura de headers e outros operadores de header continuam não suportados.

Só a primeira ocorrência de cada header participa da comparação, sem concatenar
ou separar vírgulas. Ausência faz todos os operadores falharem; presente vazio
satisfaz `exists` e `== ""`. `contains ""` passa em qualquer valor presente.

`[Asserts]` exige declaração `HTTP` na entrada. `HTTP *` é aceito com ou sem
assertions/capturas e permite qualquer status, inclusive 404/503. Sua linha mostra
`status aceito por HTTP *`; o resumo conta em `sem_validação`, não em `validadas`.
Isso dispensa **só a comparação numérica**, não headers nem capturas declarados.

Após consumo/fechamento da resposta final, a ordem é status numérico → assertions
(headers/JSON na ordem escrita) → capturas, independentemente da ordem das seções. Uma falha de header retorna 1,
localiza a assertion e esperado/recebido (ausência difere de `""`), não publica
capturas nem envia a próxima requisição. Erro estrutural tardio retorna 2 sem
qualquer envio. Escrita, consumo incompleto, timeout e cancelamento continuam 2.
Valores sensíveis de Authorization/Proxy-Authorization/Cookie/Set-Cookie são
ocultos nos dois lados, inclusive esperados literais; headers comuns usam a
proteção de segredos conhecidos. Havendo captura secreta pendente na mesma
resposta, ambos os operandos são ocultos integralmente, sem antecipar a captura.
Nenhum body ou coleção de headers é exibido.

Equivalência com Hurl 8.0.0 é limitada aos pares observados: **repetidos são listas
no Hurl**, que também interpola esperados e avalia capturas antes das assertions.
Essas divergências são deliberadas, preservando as regras da história; veja
[matriz de compatibilidade](docs/compatibility.md).

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
número, booleano, null, array ou objeto, inclusive vazios. JSONPath mantém somente
raiz `$`, propriedades por ponto e índices não negativos combináveis. Consultas
são estáticas; os esperados são literais JSON completos na mesma linha, nunca
templates: `"{{nome}}"` permanece texto mesmo com a variável definida.

A igualdade preserva tipos: `"4500"` difere de `4500`. Números são comparados
exatamente por valor (`1`, `1.0`, `1e0` são iguais), inclusive aninhados e com
expoentes grandes, sem float64 ou expansão de potências. Arrays exigem a mesma
ordem; objetos ignoram somente a ordem das chaves. Valor diferente, propriedade
ausente ou índice fora do array retorna **1**; ausência difere de null presente.
JSON vazio/inválido/adicional ou percurso por tipo incompatível retorna **2**,
como erro de avaliação JSON, não de transporte/captura. Sintaxe ou construção
não suportada em qualquer entrada rejeita o arquivo inteiro antes da rede.

A avaliação é status numérico → headers e JSON **na ordem escrita** → capturas.
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
A comparação por regex está restrita a strings selecionadas de JSON: consulte
[Validar padrões textuais](docs/subset.md#padrões-textuais--us-15--issue-36). Não há
interpolação de consultas nem padrões. Hurl 8.0.0 rejeita os literais esperados
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
**2**. Ordem status → assertions escritas → capturas e fail-fast são preservados.
Diagnósticos incluem operador/causa, consulta e localização; count mostra somente
a quantidade, não a coleção. Segredos conhecidos/derivados continuam sanitizados;
captura secreta pendente oculta integralmente ambos os operandos, inclusive contagens.

O diferencial com **Hurl 8.0.0** confirmou presença de null, vazios, false e zero,
contagem e limites dos pares compartilhados. Há diferenças deliberadas de gramática
(esperados com expoente), pré-validação de count negativo/fracionário e códigos;
veja [matriz](docs/compatibility.md). Não há count em capturas, strings/objetos,
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
literais. `matches` não se aplica a headers/body/variáveis e não suporta
interpolação ou captura por regex. A evidência específica frente a Hurl 8.0.0 e
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
