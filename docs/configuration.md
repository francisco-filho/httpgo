# Guia de configuração

A configuração YAML hierárquica é uma **extensão HTTPGo**, não parte do formato
Hurl. Este guia descreve somente as fontes disponíveis nesta revisão. Para
extrair dados de respostas, veja o [guia de captures](captures.md);
para validá-los, veja o [guia de assertions](assertions.md).

## Exemplo completo: base, ambiente e CLI

Pré-requisitos: executável `httpgo` no `PATH` (veja a [instalação](../README.md#instalação))
e uma pasta de trabalho separada do clone. Os comandos usam shell POSIX;
no PowerShell leia `$LASTEXITCODE` em vez de `echo $?`.

Tenha uma API de testes com `GET /health`, resposta **200**,
`Content-Type: application/json` e body `{"status":"ok"}` nas portas usadas
abaixo. O [dummy do repositório](dev-server.md) serve apenas a porta **8080**;
ele não fornece os destinos 8081/9090. Para executar as três variantes, é preciso
prover também esses dois destinos com o mesmo contrato, ou ajustar os endereços
para servidores de testes equivalentes. Não use credenciais reais.

Na pasta de trabalho, salve `.httpgo.yaml`:

```yaml
version: 1
defaults:
  timeout: 2.5
variables:
  base_url: http://localhost:8080
  product:
    name: Pizza
    price: 4500
  labels: [base, shared]
  optional: enabled
secrets: [auth.token]
```

Crie `environments/local.yaml`:

```yaml
defaults:
  timeout: 3
variables:
  base_url: http://localhost:8081
  product:
    name: Suco
  labels: [local]
  optional: null
secrets: []
```

Salve `exemplo.httpgo`:

```hurl
GET {{base_url}}/health
HTTP 200
[Asserts]
jsonpath "$.status" == "ok"
```

Execute **a partir da pasta que contém `.httpgo.yaml`**:

```sh
httpgo test exemplo.httpgo
httpgo test exemplo.httpgo --env local
httpgo test exemplo.httpgo --env local --var base_url=http://localhost:9090
echo $?
```

| Fonte efetiva | `base_url` | `product.name` / `product.price` | `labels` | `optional` | timeout |
|---|---|---|---|---|---|
| Só base | `http://localhost:8080` | `Pizza` / `4500` | `["base","shared"]` | `"enabled"` | 2,5 s |
| Base + local | `http://localhost:8081` | `Suco` / `4500` | `["local"]` | `null` | 3 s |
| Base + local + CLI acima | `http://localhost:9090` | `Suco` / `4500` | `["local"]` | `null` | 3 s |

Cada comando envia um GET ao respectivo destino e retorna **0**, com status 200 e
assertion aprovada, resultado em stdout e sem erro em stderr. A CLI não imprime
o body. Se a API responder 200 com `{"status":"down"}`, o resultado é **1**
(divergência); se não for possível conectar, é **2** (transporte), não falha de
configuração. O código indica o resultado real, não garante que as três portas
estejam disponíveis.

## Selecionar arquivos e ambientes

- Sem `--config`, lê somente `.httpgo.yaml` do **diretório corrente**. Não procura
  ancestrais nem o diretório do cenário. Ausência desse arquivo opcional usa os
  defaults internos. Arquivo vazio/só comentários equivale a ausência.
- `--config projeto/base.yaml` ou `--config=projeto/base.yaml` seleciona **somente**
  esse arquivo, sem combinar com `.httpgo.yaml`. Caminho explícito ausente ou
  falha de abertura/leitura/fechamento retorna **2** antes de qualquer HTTP.
- `--env local` ou `--env=local` seleciona somente
  `environments/local.yaml`, relativo ao **diretório da base selecionada**,
  inclusive quando a base é vazia. Sem base, usa o diretório corrente. Não há
  busca relativa ao cenário, fallback ou seleção automática de ambiente.
- O nome do ambiente aceita `[A-Za-z0-9_-]+`, sem pontos, espaços, Unicode ou
  separadores de caminho. Sem `--env`, nenhum ambiente é aberto. Ambiente pedido
  ausente/ilegível é erro **2**. Base tem `version: 1` obrigatória; no ambiente,
  a versão é opcional (se presente, precisa ser 1).
- Cada flag de seleção aceita uma ocorrência, antes/depois do cenário, após
  `test`. Repetição, argumento ausente ou vazio são erro **2**. Não é possível
  selecionar vários ambientes na mesma invocação.

Para mover a base do exemplo para `projeto/base.yaml`, mova também o ambiente
para `projeto/environments/local.yaml` e execute, mantendo o cenário onde está:

```sh
httpgo test exemplo.httpgo --config projeto/base.yaml --env local --var base_url=http://localhost:9090
```

Resultado: GET em 9090 e **0** com a resposta indicada. A localização da base não
muda a raiz dos payloads: `file,...;` continua relativo ao arquivo de cenário.

## Campos, tipos e limites

| Campo | Contrato |
|---|---|
| `version` | Inteiro `1`; obrigatório na base não vazia, opcional no ambiente |
| `defaults` | Mapa que aceita somente `timeout` |
| `defaults.timeout` | Número de segundos de `0.001` a `86400`, exato em milissegundos inteiros; não string, unidade textual, infinito ou arredondamento |
| `variables` | Mapa de caminhos de variáveis; valores string, número finito, booleano, `null` ou array JSON-compatível |
| `secrets` | Lista de nomes válidos que acumula proteção; não fornece valores |

O default interno de timeout é **10 segundos por requisição/tentativa**. Inclui
conexão, TLS, redirects e consumo/fechamento da resposta; cada requisição ganha
seu orçamento. Cancelamento ou prazo menor do chamador prevalece. Não é timeout
da suíte nem promessa de interromper I/O local bloqueado pelo sistema operacional.
Não há retry implícito; [polling por entrada](subset.md#polling-local-por-entrada)
é separado.

Cada arquivo YAML admite **um documento**, raiz mapa e até **1 MiB (1.048.576
bytes)**, inclusive whitespace/comentários. Campos desconhecidos, duplicações,
tipos incompatíveis, âncoras, aliases e tags explícitas (até `!!str`) são
rejeitados em qualquer profundidade. Datas precisam de aspas para serem strings;
`.inf`/`.nan` não são JSON e são rejeitados.

Cada segmento de variável segue `[A-Za-z_][A-Za-z0-9_]*`, separado por pontos.
`product: {name: Pizza}` e `product.name: Pizza` definem o mesmo caminho
`product.name`, não uma variável objeto `product`. Mapas dentro de arrays são
objetos de conteúdo, com chaves string, não novos caminhos. Strings numéricas
entre aspas mantêm tipo; números preservam precisão, inclusive aninhados, sem
arredondamento por float64.

## Precedência e conflitos

A precedência **implementada** é:

**defaults internos < YAML base < ambiente selecionado < CLI**.

O override é pelo **nome exato**, preservando folhas irmãs. No exemplo, alterar
`product.name` não apaga `product.price`. Listas e `null` substituem a folha
inteira: não há concatenação de listas nem tratamento de `null` como ausência.
Um mapa hierárquico, inclusive vazio, não pode virar valor escalar/objeto opaco,
nem um valor virar mapa com descendentes. `--var 'product={"name":"Outro"}'`
não pode substituir o mapa `product` do exemplo: conflito **2**, sem HTTP.

Cada fonte é validada independentemente **antes** do merge: override não oculta
campo desconhecido, referência malformada ou chave duplicada da base. Na mesma
fonte, `product: {name: Pizza}` junto com `product.name: Suco` é duplicação;
`product: Pizza` junto com `product.name: Suco` é conflito valor/descendente.
Ambos retornam **2** antes da execução, com arquivo/linha/coluna quando
identificáveis (fallback 1:1), sem aplicar configuração parcial.

`--var nome=valor` pode repetir para nomes diferentes. Repetir o mesmo nome é
erro **2**, mesmo com valor idêntico. Literal JSON CLI preserva tipo, outros
valores são texto; `--var 'code="001"'` força string. Objetos CLI são opacos:
`--var 'item={"id":1}'` define só `item`, não `item.id`. Nomes com conflito
ancestral/descendente são rejeitados nas duas direções.

## Referências ao ambiente do processo e segredos

Ambiente YAML nomeado **não é** ambiente do processo. `${NOME}` é uma referência
**explícita**, não override automático nem camada de precedência. Não há
mapeamento automático `HTTPGO_*`, carga automática de `.env`, entrada por stdin, hooks ou integração
direta com cofres nesta versão.

Para exercitar o exemplo seguinte com o dummy ativo em 8080, configure
externamente `OWNER_TOKEN` com o valor **fictício** `token-de-demonstracao`.
Em produção/CI, injete a credencial pelo ambiente do processo a partir de uma
fonte segura; não digite credenciais reais em comandos, flags ou `--var`.
Salve `credenciais.yaml`:

```yaml
version: 1
variables:
  base_url: http://localhost:8080
  auth:
    token: ${OWNER_TOKEN}
```

Salve `credenciais.httpgo`:

```hurl
GET {{base_url}}/health
Authorization: Bearer {{auth.token}}
HTTP 200
```

```sh
httpgo test credenciais.httpgo --config credenciais.yaml
```

Com a variável presente, envia o header e retorna **0**; o dummy ignora o header,
portanto isso testa injeção, **não autenticação**. Sem `OWNER_TOKEN`, retorna
**2 antes de parsing/HTTP**, mesmo se `auth.token` não for usado pelo cenário.
Uma referência ainda efetiva é obrigatória; todas são resolvidas antes da sessão.

A referência ocupa **todo** o valor string de uma folha de `variables`, com nome
ASCII `[A-Za-z_][A-Za-z0-9_]*`. `prefixo-${NOME}`, múltiplas referências,
delimitadores inválidos ou referências dentro de listas/objetos nelas contidos
são rejeitados por fonte, mesmo com override posterior. Outros campos não
resolvem referências. `Bearer` pertence ao template do cenário.

Primeiro valida/combina fontes e overrides, depois consulta somente referências
**efetivas**. Uma referência válida substituída não exige a variável de processo
nem consulta seu valor descartado. Presença vazia é string vazia, não ausência.
Todo valor lido é string: `123`, `true`, `null` e `{"x":1}` não viram tipos JSON.
Conteúdo `${OUTRA}` ou `{{outra}}` não é reinterpretado.

### Proteção preservada após override

- Cada destino de referência é marcado como secreto **antes** dos overrides.
  `secrets` e `--secret nome` acumulam marcas, inclusive de capturas futuras;
  marcação não cria valor nem permite usar um valor ausente.
- Uma lista `secrets: []` posterior não remove proteção. Renovação/override por
  nome também não remove a marca. Só um substituto fictício/não sigiloso pode
  ser fornecido em `--var`, nunca uma credencial real.
- Valores conhecidos da base, ambiente e substitutos, inclusive sobrescritos,
  variantes e constituintes JSON conhecidos, são protegidos. Referência descartada
  não é consultada só para descobrir a grafia de um segredo desconhecido.
  Capturas/derivados acrescentam proteção durante a execução.
- Proteção de stdout/stderr/relatórios não protege histórico, argumentos na
  listagem de processos, arquivos em disco ou acesso privilegiado ao ambiente.
  Não há detecção universal de segredos arbitrários não marcados nem de
  transformações/codificações desconhecidas. Não coloque segredos em paths.
- Use permissões `0600` para arquivos sensíveis e `0700` para diretórios em POSIX;
  no Windows restrinja ACLs de modo equivalente. Prefira não gravar credenciais
  reais no YAML. Cada invocação tem configuração/contexto novos.

## Arquivo local explícito — extensão HTTPGo

`--env-file <arquivo>` ou `--env-file=arquivo` adiciona uma fonte **somente para
referências explícitas `${NOME}`** na base ou no ambiente YAML. Aceita uma única
ocorrência em `test`, para cenário individual ou suíte. O caminho relativo parte
sempre do **diretório corrente**, independentemente de `--config`, `--env` ou do
cenário; caminhos absolutos também são aceitos. `--env` continua selecionando
somente um ambiente YAML. Sem `--env-file`, nenhum `.env` é carregado ou procurado,
nem no cwd, nem em ancestrais.

Salve `.env.local` (somente valores fictícios neste exemplo):

```dotenv
API_URL=http://localhost:8080
OWNER_TOKEN=token-de-demonstracao
```

Salve `local.yaml`:

```yaml
version: 1
variables:
  base_url: ${API_URL}
  auth:
    token: ${OWNER_TOKEN}
```

Salve `profile.httpgo`:

```hurl
GET {{base_url}}/profile
Authorization: Bearer {{auth.token}}
HTTP 200
```

```sh
httpgo test profile.httpgo --config local.yaml --env-file .env.local
httpgo test tests/ --config local.yaml --env-file=.env.local
```

Com uma API `GET /profile` que retorna 200, o primeiro comando envia URL e header
e retorna **0**. O segundo usa a mesma fonte inicial em todos os arquivos da
suíte, mantendo capturas independentes. O dummy não implementa `/profile`; para
exercitar injeção com ele, use `/health` (não comprova autenticação).

Após base < ambiente YAML < CLI, cada referência ainda efetiva consulta primeiro
o **ambiente do processo**, depois o arquivo. Presença no processo **inclusive
vazia** prevalece: vazio não ativa fallback. Referência substituída por YAML/CLI
não exige valor nas fontes. Todo resultado é string, inclusive `123`, `true` e
`null`, sem segunda expansão. Chaves extras não criam variáveis de cenário,
`HTTPGO_*` ou opções como `NO_COLOR`. A carga não altera o ambiente do processo.

### Perfil literal aceito

- Nome ASCII `[A-Za-z_][A-Za-z0-9_]*`, seguido de `=` e valor; somente o primeiro
  `=` separa nome/valor. Espaços externos são removidos; vazio é válido.
- Linhas vazias e comentários de linha inteira iniciados por `#` são aceitos.
  LF e CRLF são terminadores. Arquivo vazio/só comentários é válido.
- Aspas simples ou duplas podem delimitar **todo** o valor; somente os
  delimitadores são removidos, preservando espaços internos.
- Não há escapes, expansão, shell ou comandos: `#` após `=` é texto,
  `${OUTRA}`, `{{outra}}`, `$()` e `\n` são literais. Por exemplo,
  `TEXT='  a=b#c\n  '` conserva o conteúdo e seus espaços internos.
- Nomes repetidos (mesmo com valor igual), nomes inválidos, linha sem atribuição,
  aspas não fechadas, texto após a aspa final, prefixo `export` e valores
  multilinha são rejeitados. Não é o perfil ampliado de bibliotecas dotenv.

Arquivo solicitado inexistente/ilegível/inválido e flag repetida/sem argumento/
vazia retornam **2 antes de HTTP**. Todo o arquivo é validado, mesmo se valores
forem substituídos pelo processo ou não utilizados. Não há publicação parcial.
Referência efetiva ausente em ambas as fontes também falha antes de HTTP.
Diagnósticos mostram causa controlada e arquivo/linha quando disponíveis, nunca
valores, linhas do arquivo ou erros brutos de I/O. O limite de 1 MiB do YAML não
é aplicado a essa fonte; não há teto de tamanho definido para `.env`.

### Segredos e isolamento

A proteção da US-13 é preservada integralmente: destinos de `${NOME}` são
marcados automaticamente antes de overrides; substitutos e derivados conhecidos
continuam protegidos em resultados, texto, verbosidade, JUnit e HTML. Esta fonte
não classifica segredos novos nem amplia os limites de detecção. Credenciais
reais continuam proibidas em flags/`--var`; podem vir do processo ou deste arquivo
explicitamente selecionado. Não versione arquivos com credenciais reais e use
permissões `0600`/diretórios `0700` (ACLs equivalentes em Windows). Sanitização de
saídas não protege arquivos no disco, argumentos ou transformações desconhecidas.

O arquivo é carregado uma vez por invocação e fechado antes de HTTP. Próxima
invocação recarrega a fonte, sem cache de valores, proteção ou capturas anteriores.
Cancelamento é verificado durante preparação; não promete interromper I/O local
bloqueado pelo SO. Sem descoberta automática, múltiplos arquivos por invocação,
importação automática, sintaxe shell ou expansão recursiva.

## Resultados e referências

**0**: validações aprovadas; **1**: somente divergência de assertion/status;
**2**: uso, configuração, entrada, captura, renderização, avaliação, transporte,
cancelamento ou escrita. Em suíte, 2 prevalece sobre 1. Resultados vão para stdout,
diagnósticos seguros para stderr. Falha de configuração impede todo HTTP;
falha durante execução preserva resultados anteriores, sem desfazer efeitos na API.

Contratos detalhados: [YAML](subset.md#extensão-httpgo-configuração-yaml),
[ambientes](subset.md#extensão-httpgo-ambientes-nomeados),
[referências de processo](subset.md#extensão-httpgo-referências-ao-ambiente-do-processo)
e [compatibilidade](compatibility.md). Os guias consolidam o comportamento atual,
sem anunciar fontes ou recursos futuros.
