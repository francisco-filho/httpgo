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

### Baixar e instalar uma release

A distribuição pelo terminal tem como alvos **Linux e macOS (Darwin)**, nas
arquiteturas **amd64** (`x86_64`) e **arm64** (`aarch64`/`arm64`). O instalador
seleciona o pacote da máquina; Windows não faz parte desta distribuição.
Pré-requisitos: **Bash 3.2 ou superior, curl, tar com suporte a gzip e um
verificador SHA-256 (`sha256sum` ou `shasum`)**, além dos utilitários usuais do
sistema (`uname`, `mktemp`, `mkdir`, `cp`, `chmod`, `mv` e `rm`). O executável é
independente: não exige Go, CGO ou outro runtime na máquina de destino.

Os comandos públicos abaixo dependem de uma release com os assets em
[francisco-filho/httpgo](https://github.com/francisco-filho/httpgo/releases).
**A publicação pública e a execução nativa nas quatro plataformas ainda não
foram homologadas nesta revisão**; veja as [evidências e pendências](docs/validation.md).
Enquanto não houver release disponível, use a compilação por fonte abaixo.

Para baixar e instalar a última release estável, sem rascunhos ou pré-releases:

```sh
curl -q -fsSL --proto '=https' --proto-redir '=https' \
  https://github.com/francisco-filho/httpgo/releases/latest/download/install.sh | bash
"$HOME/.local/bin/httpgo" --version
```

O destino padrão é `~/.local/bin`. O instalador informa versão e caminho; se
necessário, orienta a adicionar esse diretório ao `PATH`. Ele **não usa sudo nem
altera configurações do shell**. Você pode usar o caminho completo do executável
sem modificar o `PATH`.

Para fixar uma versão, substitua `v1.0.0` por uma tag publicada. A seleção é exata,
sem fallback; uma pré-release como `v1.0.0-rc.1` também pode ser escolhida:

```sh
curl -q -fsSL --proto '=https' --proto-redir '=https' \
  https://github.com/francisco-filho/httpgo/releases/latest/download/install.sh | bash -s -- --version v1.0.0
```

Para escolher outro diretório, inclusive um caminho com espaços:

```sh
curl -q -fsSL --proto '=https' --proto-redir '=https' \
  https://github.com/francisco-filho/httpgo/releases/latest/download/install.sh | bash -s -- --bin-dir "$HOME/meus binarios"
"$HOME/meus binarios/httpgo" --version
```

**O comando com pipe executa código remoto na sua máquina.** Se preferir baixar e
inspecionar o script antes de executá-lo, faça as etapas separadamente:

```sh
curl -q -fsSL --proto '=https' --proto-redir '=https' \
  https://github.com/francisco-filho/httpgo/releases/latest/download/install.sh -o install.sh
# Abra install.sh em um editor e revise seu conteúdo antes de continuar.
bash ./install.sh --version v1.0.0
```

Para obter também o script de uma tag específica, use
`https://github.com/francisco-filho/httpgo/releases/download/v1.0.0/install.sh`.
Os downloads exigem HTTPS com verificação de certificado. Após selecionar a tag,
o instalador baixa o pacote e `checksums.txt` **da mesma release** e verifica
SHA-256 antes de gravar o executável. O checksum verifica **integridade**, mas
**não é uma assinatura de autenticidade**. Cada release reúne `install.sh`,
`checksums.txt` e os pacotes `httpgo_linux_amd64.tar.gz`,
`httpgo_linux_arm64.tar.gz`, `httpgo_darwin_amd64.tar.gz` e
`httpgo_darwin_arm64.tar.gz`, cada um contendo apenas `httpgo`.

Para reinstalar, repita o comando: um `httpgo` existente no destino é substituído
sem confirmação, somente após download, validação e preparação completos. Falhas
de rede, integridade, extração ou gravação retornam status diferente de zero sem
truncar a instalação anterior. Plataforma não suportada, tag inexistente ou pacote
ausente produzem erro, sem instalar outra versão. O instalador não realiza
atualização automática, desinstalação nem integração com gerenciadores de pacotes.

### Compilar a partir do código-fonte

Como alternativa independente da publicação, tenha Git e **Go 1.26.10**, conforme
[go.mod](go.mod):

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
Os dois comandos de ajuda exibem o mesmo resumo de uso, opções e exemplos.
Para tutoriais e detalhes, continue neste guia ou consulte a
[referência do subconjunto](docs/subset.md#cli-e-saída), que define as capacidades desta revisão.

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

Com stdout e stderr em terminal interativo, o progresso em stderr termina em
`- exemplo.httpgo  OK  1/1`, e stdout recebe o resumo da suíte. O código é **0**.
Se qualquer um dos streams for redirecionado, preserva-se o texto detalhado:
stdout informa o GET, status 200 e resumo com uma requisição validada, sem
diagnóstico de erro em stderr. Por padrão, a CLI não imprime o body; `-vv`
permite prévias JSON protegidas somente nos steps com falha, como explicado abaixo.
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
O compacto usa cores quando `NO_COLOR` está ausente; definir a variável, mesmo
vazia, remove cores sem remover a barra textual e suas atualizações. Ajuda e
versão permanecem estáticas e sem cores. Consulte os
[contratos da CLI](docs/subset.md#cli-e-saída).

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

O [guia de assertions](docs/assertions.md) reúne o catálogo completo por status,
JSONPath, header, body e extração regex, com sintaxe, exemplos e resultados.
Explica tipos, ausência/vazio/`null`, contagem, perfil literal versus
`--render-asserts` e a diferença entre divergência (**1**) e avaliação inválida (**2**).

O [guia de captures](docs/captures.md) mostra como extrair valores JSON tipados,
contagens, headers e grupos regex, e reutilizá-los no mesmo arquivo. Inclui
exemplos completos, ausência versus vazio/`null`, operandos dinâmicos com opt-in,
renovação exata, confirmação do lote e sigilo. Capturas não passam entre arquivos;
falha de captura é **2**, diferente de divergência de assertion (**1**).

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

No primeiro comando, **somente quando stdout e stderr são TTY**, cada arquivo
ocupa uma linha em stderr, apenas com nome-base e extensão. A linha atual começa
em `0/N` após o parsing completo e avança por step definitivo (requisição com
capturas/validações); polling não gera steps extras. Linhas finalizadas permanecem,
sem repetição. Por exemplo, durante a execução:

```text
- 01-health.httpgo  OK  1/1
- 02-produto.httpgo  |=====>       1/2
```

Ao finalizar, a barra vira `OK` ou `ERRO`. O diagnóstico seguro fica imediatamente
abaixo do erro, com step e posição disponíveis. Falha de carga/parsing tem total
`?`; arquivos não alcançados mostram `NÃO EXECUTADO 0/?`, sem parsing antecipado.
Nomes iguais em diretórios diferentes continuam em linhas distintas.

Em erro, o bloco separa localização, etapa e expressão, sem repetir o caminho:

```text
- 27-phase-six-advanced-menu.httpgo  ERRO  7/15
  step 7 - Linha 114:1
  [assertion]
  jsonpath "$.orders[?(@.number==[literal oculto])].items[0].allergens" contains
     expected "SOY" => received tipo array
- 29-phase-seven-combos.httpgo  OK  13/13
```

A última linha do diagnóstico fica vermelha quando as cores estão habilitadas.
A etapa é `[assertion]`, `[captures]` ou `[status]`, conforme a falha; erros de
carga/parsing, preparação e HTTP usam `[parsing]`, `[renderização]` e `[transporte]`.
Step, posição e expressão só aparecem quando existem metadados seguros disponíveis;
não se recupera a linha bruta do cenário nem se imprime body para detalhar o erro.

O modo compacto mantém somente a linha de progresso e os diagnósticos: **não**
exibe balanço ou separador por cenário. Os nomes ficam em negrito quando ambos
os streams são TTY e `NO_COLOR` está ausente.

Somente o resumo final vai a stdout no modo compacto. **Todos os modos de texto**
(compacto, detalhado redirecionado, `-v` e `-vv`) usam o mesmo resumo. Exemplo
ilustrativo de dois cenários, cada um com um step, concluídos em seis segundos:

```text
Cenários executados: 2
Tempo médio por Cenário: 3s
Sucesso/erros: 2/0

Testes executados: 2
Tempo médio por Teste: 3s
Sucesso/erros: 2/0

Tempo: 6s
```

“Cenários executados” conta arquivos cuja carga foi tentada, inclusive erro de
leitura/parsing. “Testes executados” conta steps com desfecho definitivo, inclusive
erros de preparação antes do HTTP, transporte, captura e validação. Polling conta
um único teste; steps não alcançados e falhas de carga/parsing não inventam testes.
Cada grupo tem seu próprio `Sucesso/erros`: cenários aprovados/não aprovados e
steps aprovados/com falha ou erro. Cada média divide a duração total da suíte
pela contagem correspondente, ou é `não aplicável` com zero. `Não executados: N`
identifica somente cenários pulados, quando positivo. O tempo total vem por último.
Erros globais anteriores à execução não produzem resumo; JUnit e HTML não mudam.

Redirecionar stdout ou stderr, sem verbosidade, mantém os resultados e contadores
por arquivo detalhados em stdout e diagnósticos em stderr, sem atualização de linha
ou cores. O resumo final acima é uma **mudança deliberada de contrato do texto**,
inclusive redirecionado; execução e exit codes não mudam.
A proteção incremental de segredos conhecidos/derivados cobre nomes e diagnósticos,
mas não apaga texto exibido antes de um segredo ser conhecido.

### Investigar falhas com progresso verboso

```sh
httpgo test suite/ --var base_url=http://localhost:8080 -v
httpgo test suite/ --var base_url=http://localhost:8080 -vv
```

`-v` ou `--verbose` seleciona nível 1. `-vv`, `-v -v`, duas ocorrências de
`--verbose` ou uma combinação curta/longa selecionam nível 2. Mais de duas
ocorrências, valores como `--verbose=2` e combinação com `--output junit` ou
`--output html` são uso inválido **2**, antes de HTTP ou criação de relatório.
Sem essas flags, a seleção compacto/legado descrita acima permanece inalterada.

Nos dois níveis, **stderr** anuncia cada nome-base e extensão antes de carregar
o arquivo e apresenta uma linha estática por step definitivo, inclusive em pipes:

```text
01-health.httpgo
[OK] GET http://localhost:8080/health (12ms)
Sucesso: 1
-----------------------------------------------
02-produto.httpgo
[ERRO] GET http://localhost:8080/products/1 (20ms)
  step 1 - Linha 2:1
  [status]
  HTTP 200
     expected 200 => received 503
Sucesso: 0 - Erro: 1
-----------------------------------------------
```

Os tempos são ilustrativos: incluem preparação, todas as tentativas e esperas.
Polling não imprime divergências transitórias. `[OK]` exige concluir as capturas
e validações declaradas, mas não inventa validação de status para `HTTP *` ou
status ausente. Falhas de parsing, preparação, transporte, status, assertions e
capturas conservam categoria/localização disponíveis, antes do próximo arquivo.
Arquivos não alcançados aparecem como `NÃO EXECUTADO`, sem parsing antecipado.
Somente o resumo final de cenários e testes vai a **stdout**, também com redirecionamento.
**Somente nos modos verbosos**, o balanço de cada cenário fica em stderr, após
seus diagnósticos/prévias, antes do próximo nome: `Sucesso: N`, acrescentando
` - Erro: N` somente quando há steps com falha/erro, seguido de 47 hífens. Não há barra ou atualização de linha. Cores e nomes em negrito exigem
ambos os streams TTY e `NO_COLOR` ausente; rótulos permanecem sem cores.

**Exceção opt-in à ausência de bodies:** somente `-vv` acrescenta `[request body]`
e `[response body]` no step com falha/erro. São prévias da última tentativa
enviada e de seu salto final, sem histórico, releitura de arquivo ou template não
enviado. 301/302/303 convertendo POST em GET mostram request ausente; 307/308
preservam seu body. São exibidos apenas JSON completo UTF-8 (inline ou externo),
sem conversão de charset. Binários, multipart e conteúdo não JSON são omitidos.
Ausência é indicada como `ausente`; falha pré-envio ou resposta parcial como
`indisponível/incompleto`, nunca como `{}` artificial.

Cada prévia é protegida **antes** do limite de **16 KiB (16.384 bytes) por body**,
com corte UTF-8 e indicação explícita de truncamento. A representação pode ser
normalizada para proteger escapes JSON equivalentes, e ocultação/corte podem
invalidar sua sintaxe exibida. Captura secreta ainda não extraída omite toda a
resposta por proteção; extração bem-sucedida registra o segredo mesmo se o lote
não for confirmado. A prévia não antecipa capturas nem altera os dados enviados.
Marque segredos com `--secret`/YAML ou use referências ao ambiente: **não há
promessa de detectar credenciais arbitrárias não marcadas ou codificações
desconhecidas**. O opt-in pode reter transitoriamente o body final completo em
memória; 16 KiB é limite de exibição, não de consumo HTTP. Sucessos, nível 1,
modo padrão e relatórios continuam sem bodies. Falhas de escrita mantêm código
**2**, sem novos envios ou repetição no writer falho.

Nos comandos de relatório, stdout fica vazio: JUnit grava `target/test-results.xml`, substituindo destino existente;
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

Os guias reúnem receitas completas, com pré-requisitos de API, entrada/resposta,
comando e resultado esperado. Algumas requerem outra API de testes, não o dummy:

- [Cache condicional com ETag](docs/captures.md#header-literal) e
  [cookie explícito](docs/captures.md#extrair-cookie-explicitamente).
- [Projeção e filtros JSONPath](docs/captures.md#jsonpath-tipos-e-seleção).
- [Guardar e comparar uma contagem](docs/captures.md#capturar-contagem-e-reutilizar-em-assertion).
- [Capturar e validar um código textual](docs/assertions.md#capturar-e-validar-o-código-na-mesma-resposta).
- [Comparar valores anteriores](docs/assertions.md#comparar-com-valores-anteriores) e
  [rotação de token](docs/assertions.md#desigualdade-e-rotação-de-token).

Não rode toda `testdata/scenarios` como suíte: ela contém casos intencionalmente
inválidos e não suportados.

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
