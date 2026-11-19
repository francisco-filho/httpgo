# Subconjunto suportado e extensões — US-01 a US-17, US-20 a US-36, US-38

## Extensão padrão dos cenários — US-38 / issue #79

Use `.httpgo` para novos cenários. A linguagem é a mesma do subconjunto descrito
neste documento; a extensão não habilita recursos. Guias atuais:
[configuração](configuration.md), [captures](captures.md) e [assertions](assertions.md).
Para aceitação de cenários
legados e exemplo de suíte mista, veja
[compatibilidade das extensões](compatibility.md#extensões-de-arquivo--us-38--issue-79).

<a id="assertions-sobre-extração-regex"></a>

## Assertions sobre extração regex — US-36 / issue #74

Ampliação explícita: `[Asserts]` aceita a query `regex "padrão"` com `==`,
`contains`, `not contains`, `matches` ou `exists`. Exige `HTTP <status>` ou
`HTTP *`. Os quatro primeiros recebem exatamente uma string JSON entre aspas
duplas; `exists` não recebe operando. Strings/escapes e regex seguem a US-15:
`"\\d"` entrega `\d` à regex; `"\x41"` é inválido como JSON. Sem delimitadores
`/.../`, lookaround, backreferences ou pipelines.

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

Execute `httpgo test testdata/scenarios/scenario_regex_assertion_signup.httpgo --var mailbox_url=http://localhost:8025 --var base_url=http://localhost:8080 --secret signup_code`.
Requer endpoints que retornem `Seu código de verificação é: 123456` e aceitem
`{"code":"123456"}`: **0**, sem revelar o código; esperado com sete dígitos:
**1**, sem POST. [Cenário versionado](../testdata/scenarios/scenario_regex_assertion_signup.httpgo).

A consulta usa o body **final completo**, consumido/fechado e decodificado pelas
regras US-21 (UTF-8 estrito por padrão, charset declarado), sem exigir JSON nem
normalizar texto. Seleciona como string apenas o primeiro grupo da primeira
correspondência; não procura grupo/correspondência posterior, não publica variável
nem altera capturas. Query e predicado seguem posições da fonte LF/CRLF.

| Seleção | `==`, `contains`, `matches` | `exists` | `not contains` |
|---|---|---|---|
| Grupo presente | comparação textual, sem coerção | aprova | nega substring |
| Sem match ou primeiro grupo opcional não participante | reprova 1 | reprova 1 | aprova |
| Grupo participante vazio | `== ""`, `contains ""`, `matches "^$"` aprovam | aprova | `not contains ""` reprova 1 |

Expressão válida **sem grupo** e body indecodificável são avaliação **2**, também
com negação. Captura regex continua estrita: ausência é erro de captura **2**.
Regex estática malformada/não suportada, escape/sintaxe inválida ou construção
excluída em qualquer entrada bloqueia **todo HTTP** do arquivo com erro localizado
**2**, distinguindo sintaxe inválida de construção não suportada.

Extração é sempre **estática**, mesmo com `--render-asserts`. A flag só resolve
esperados desta query, com iniciais/confirmados/provisórios atuais; valida tipo
textual e `matches` final **antes de selecionar/decodificar a assertion alcançada**.
Ausência de variável, array/objeto textual ou regex final inválida é **2**, não
aprovação por negação. Inserções não são templates novos. Sem flag, esperados
são literais. Regex `matches` sem referências compila antes de HTTP.

Preserva US-34: status → todas as capturas provisórias → assertions ordenadas →
confirmação com contexto ativo. Falha descarta o lote e impede a próxima entrada.
Polling só retenta divergência **1**; reenvia com valores confirmados anteriores,
sem herdar capturas reprovadas. Erro **2** encerra. Texto decodificado é reutilizado
entre assertions body/regex da mesma resposta, nunca entre tentativas/arquivos.

Diagnóstico distingue grupo ausente, divergência de conteúdo e erro, com predicado
e localização seguros; **nunca body nem grupo selecionado**. Padrões/esperados
protegidos ficam ocultos em texto, callbacks e JUnit, inclusive em lotes descartados.
Metadados não retêm padrões brutos, regex compiladas ou templates. Marque capturas
secretas; valores arbitrários não marcados/transformações desconhecidas não têm
garantia de detecção. Falhas de saída/escrita curta/close prevalecem com **2**,
sem novos envios/reexecução ou anúncio falso. JUnit mantém um testcase por arquivo.

Continuam excluídos `variable`, `body matches`, regex de header como query de
assertion, escolha de grupo, correspondência inteira implícita, extração dinâmica,
pipelines e novos predicados. YAML hierárquico/interpolação externa são extensões
HTTPGo. [Diferenças Hurl 8.0.0](compatibility.md#assertions-sobre-extração-regex--us-36--issue-74)
e [arquitetura](architecture.md#assertions-sobre-extração-regex--us-36--issue-74).

<a id="renovação-transacional-de-capturas"></a>

## Renovação transacional de capturas — US-34 / issue #70

**Quebra deliberada de contrato**, padrão sem nova flag. Substitui restrições de
ordem e colisão exata das histórias anteriores, inclusive US-26/30/31/33.

1. Consumir/fechar a resposta completa e validar status, quando declarado.
2. Extrair todas as capturas usando **somente iniciais/confirmados anteriores**.
   Novas capturas do lote não podem encadear consultas. Validar todo o lote antes
   de expô-lo; qualquer captura inválida é **2**, antes de qualquer assertion.
3. Avaliar assertions na ordem escrita sobre a visão provisória. O lote atual
   prevalece sobre confirmado e inicial, independentemente da ordem das seções.
   Sem `--render-asserts`, esperados continuam literais e consulta dinâmica é
   rejeitada antes de HTTP; a nova ordem não ativa interpolação implicitamente.
4. Com status/assertions aprovados e contexto ativo, confirmar todo o lote.
   Entradas sem assertions confirmam após status aceito; sem capturas usam a
   sessão confirmada. `HTTP *`/omissão de status mantêm seus significados.

Nome **exatamente igual** pode substituir variável inicial ou captura anterior,
inclusive mudando tipo. Mantêm-se precisão, `null`, vazios, restrições textuais e
JSON completo. Objetos permanecem opacos, sem variáveis por propriedade. Duplicação
no lote e conflito ancestral/descendente, nas duas direções, continuam **2**. Não
muda a validação de variáveis duplicadas em CLI/YAML. Comparar antigo e novo exige
nomes distintos. Cada arquivo/executação começa com o contexto inicial imutável.

Divergência, captura/avaliação inválida, timeout ou cancelamento antes da confirmação
preserva todos os valores anteriores e a ausência dos nomes novos. Sem rollback
de efeitos na API. Polling reenvia com valores confirmados antes da entrada;
somente sucesso confirma. Divergência **1** permite retry no orçamento US-26,
erro **2** encerra. Status divergente é barreira anterior às capturas.

Proteção é monotônica: marcas e valores secretos anteriores, novos e derivados
sobrevivem ao descarte e à troca de arquivo da invocação, sem tornar capturas
resolvíveis em outros arquivos. Texto, callbacks e JUnit recebem apenas resultados
seguros. Segredos arbitrários não marcados/codificações desconhecidas permanecem
fora da garantia; valores iniciais secretos vêm do ambiente, nunca de flags.

Exemplo completo: [scenario_capture_renewal.httpgo](../testdata/scenarios/scenario_capture_renewal.httpgo).
Execute `httpgo test testdata/scenarios/scenario_capture_renewal.httpgo --render-asserts --var base_url=http://localhost:8080 --var job_id=inicial`.
A API deve fornecer `/jobs/current` com `selectedId: "job-a"` e `jobs` com esse ID
`READY`, depois `/jobs/job-a/successor` com `{"id":"job-b","status":"READY"}`
e `/jobs/job-b` com 200. Sucesso **0**; selecionado não pronto **1**, sem segundo
envio. Inverter `[Asserts]`/`[Captures]` não altera o resultado.

Não habilita query `variable`, novos predicados, encadeamento do próprio lote,
sobrescrita hierárquica, nova política de retry/configuração/entrada de credenciais.
YAML hierárquico e interpolação externa continuam **extensões HTTPGo**.
[Diferencial Hurl 8.0.0](compatibility.md#renovação-transacional-de-capturas--us-34--issue-70)
e [arquitetura](architecture.md#renovação-transacional-de-capturas--us-34--issue-70).


Este documento define somente o que o binário implementa nesta história.
Não implica suporte ao formato Hurl completo. As descrições de esperados
literais das US-07/08/14/15/21/23/24 são o **perfil padrão**. A seção
[US-30](#esperados-dinâmicos--us-30--issue-60) define sua evolução opt-in com
`--render-asserts`. A US-31 também permite operandos dinâmicos nos filtros por
igualdade de assertions e capturas, sem ampliar operadores ou passos.

## Relatório HTML — US-35 / issue #72

```bash
httpgo test tests/ --fail-fast --output html --output-file target/report.html
```

Formato de apresentação próprio do HTTPGo, sem nova sintaxe de cenário e sem
compatibilidade anunciada com flags HTML do Hurl. Aceita arquivo único e suíte,
exige destino antes de qualquer HTTP e mantém stdout vazio. Cria pais quando
possível; nome-base recebe timestamp UTC `YYYYMMDDTHHMMSSmmmZ` antes da extensão,
com `-1`, `-2` etc. na primeira colisão livre, usando criação exclusiva e sem
sobrescrever artefatos. Informa o caminho efetivo em stderr protegido só após
escrita e fechamento bem-sucedidos; falha de confirmação retorna **2**.

HTML autossuficiente offline, tema somente claro, títulos, tabelas e blocos
monoespaçados. Resumo agregado, arquivos e detalhes por requisição alcançada com
método, URL protegida, estado textual, duração lógica e diagnósticos localizados.
Duração inclui preparação/tentativas/esperas, sem apresentação, sem mudar métricas
ou execução. Requisições interrompidas ficam nas contagens conhecidas; arquivos
não analisados têm totais desconhecidos, nunca aprovações fictícias. Status
validado não significa sucesso completo; ausência de assertion não é validação.

Sem bodies/headers integrais; texto escapado, sem scripts ou recursos externos.
Proteção final anterior ao escaping cobre segredos descobertos em arquivos
posteriores; stderr já emitido não pode ser redigido retroativamente. Segredos
arbitrários não marcados/transformações desconhecidas não são detectados.
Códigos **0/1/2** e precedência preservados. Gera artefato dos resultados disponíveis
mesmo em falha/cancelamento durante a suíte, sem reenvios. Erro global anterior à
execução não abre relatório. Falha de geração/escrita/close retorna **2**, com
diagnóstico seguro e sem confirmação falsa; pode deixar arquivo parcial.

[Contrato completo, nomeação e segurança](html.md) e
[exemplo executável](../testdata/examples/html_suite/). Sem tema escuro, filtros,
histórico, dashboard, publicação externa ou HTML/JUnit simultâneos.

## Relatório JUnit — US-17 / issue #68

```bash
httpgo test tests/ --output junit --output-file target/test-results.xml
```

- `--output` aceita `junit` ou `html` (US-35 acima); sem a flag, preserva o texto
  existente em stdout. JUnit exige `--output-file` não vazio. Ambas aceitam `--flag=valor` ou
  `--flag valor`, em qualquer ordem após `test`, uma ocorrência de cada.
  Formato desconhecido, repetição ou destino sem formato retornam **2** antes
  de qualquer HTTP. `--output text/json`, JUnit em stdout e destino do modo texto
  não são suportados. Nenhuma flag nova altera a linguagem dos cenários.
- stdout fica vazio em JUnit; stderr mantém os mesmos diagnósticos seguros do
  modo texto. Abre/cria/trunca o destino somente após obter resultados da suíte,
  cria diretórios pais quando possível. Falha global de configuração, composição,
  cancelamento prévio ou descoberta não cria nem altera o destino anterior.
- Raiz `testsuite` com `name="httpgo"`, atributos `tests`, `failures`, `errors`,
  `skipped` e `time` em segundos decimais. Um `testcase` por arquivo descoberto,
  na ordem da suíte, com `classname="httpgo"`, `name` igual ao caminho normalizado
  da descoberta (relativo ao cwd, fallback absoluto entre unidades) e `time` de
  execução do arquivo. Identidade estável enquanto caminhos e cwd não mudam,
  sujeita à sanitização de segredos. Não conta requests/tentativas como testcases.
- Resultado completo aprovado não recebe filho de falha. Divergência final de
  status/header/JSON/body recebe `failure`; entrada/captura/renderização/avaliação/
  transporte recebe `error`. Não executados por fail-fast, cancelamento ou erro
  global de apresentação recebem `skipped` com tempo zero, nunca sucesso.
  Diagnósticos identificam arquivo/linha/coluna da requisição/assertion quando
  disponíveis. `message` é a categoria; texto do elemento contém o diagnóstico.
- Contagens são de arquivos; duração da suíte inclui carga/execução/apresentação,
  não a escrita posterior do XML. Um arquivo com múltiplas requisições ou polling
  gera um único testcase, com o desfecho definitivo já decidido pela suíte.
- XML usa escaping da biblioteca padrão e substitui caracteres inválidos em XML
  por U+FFFD. Nomes/diagnósticos são sanitizados **antes** da serialização e mais
  uma vez ao fim da invocação, inclusive por segredos descobertos em outro arquivo.
  Não há `system-out`, bodies, coleções de headers ou erros brutos de I/O.
  stderr incremental não pode ocultar retroativamente um valor ainda desconhecido
  como segredo; use as marcações/referências explícitas existentes.
- Artefato é produzido também com falhas: **0** sucesso, **1** assertion,
  **2** outros erros. Falha de criação/escrita curta/escrita/fechamento prevalece
  com **2**, sem anúncio de publicação. Pode deixar um arquivo parcial: geração
  não é transacional. Use um destino de relatório separado dos arquivos de entrada.
- Não reenvia requests, não reavalia assertions nem muda sessões/polling.
  [Exemplo mínimo](../testdata/scenarios/scenario_junit.httpgo) e
  [consumidor de referência/CI](junit.md). Sem dashboard, histórico persistente,
  formatos simultâneos ou envio para serviços externos.

<a id="captura-de-contagem"></a>

## Captura de contagem — US-33 / issue #66

Em `[Captures]`, `nome: jsonpath "consulta" count` é a única transformação de
contagem suportada. Sem transformação, a captura JSONPath não muda. Exemplo
[versionado](../testdata/scenarios/scenario_count_unchanged.httpgo):

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

Execute `httpgo test unchanged.httpgo --render-asserts --var base_url=http://localhost:8080`.
Quantidade preservada passa com **0**, alterada falha com **1**; mudar os itens
sem mudar a quantidade continua passando. Nenhuma coleção é impressa.

Contrato:

- A consulta seleciona primeiro, só depois `count` conta elementos do **array
  final**. Inclui elementos `null`; array vazio publica `0`. O valor publicado
  é número JSON inteiro não negativo, nunca string nem float64. Reutilização
  em JSON completo mantém número; em URL/header/string JSON usa forma textual.
- Ausência não é array vazio. `null`, string, número, booleano e objeto não são
  arrays. Esses casos, JSON inválido/adicional e percurso não avaliável falham
  com **erro de captura 2**, sem publicar qualquer parte do lote ou avançar.
  A classificação de `count` em assertion não muda: ausência/tipo não array
  falham com **1**, erro de decode/percurso com **2**.
- Cardinalidade US-27/28: zero resultados é ausência; um escalar conserva tipo e
  falha; vários valores formam array e contam seus elementos. Um array único,
  inclusive vazio, conserva seu conteúdo; vários arrays contam como elementos,
  sem achatamento, deduplicação ou contagem das correspondências internas.
- Captura/reutilização em requisições não exige opt-in. Esperado dinâmico
  `count == {{orders_before}}` exige **`--render-asserts`**, extensão explícita
  HTTPGo da US-30, também para `>`, `>=`, `<`, `<=`. Sem flag, esperado bare
  dinâmico é rejeitado antes de qualquer HTTP. A captura não ativa a flag.
- Status → seleção/transformação de capturas → assertions na ordem escrita →
  confirmação atômica → apresentação (US-34). Capturas consultam somente valores
  anteriores; assertions veem o lote atual. Nome exato é renovável; duplicação
  no lote e conflito hierárquico rejeitam tudo. Configuração inicial não muda.
- `--secret orders_before` apenas marca o nome. Quantidade descoberta é protegida
  antes de outra captura falhar; comparação posterior oculta esperado e real,
  inclusive diferentes. Captura secreta pendente oculta sem avaliação antecipada.
  Diagnóstico identifica consulta/`count`/causa/localização, não a coleção nem
  quantidade protegida. Marque segredos explicitamente: não há detecção universal.
- Só resposta final completa/fechada pode gerar baseline; leitura, fechamento,
  timeout/cancelamento e falha/short write interrompem. Escrita tem prioridade
  **2**. Sessões/arquivos independentes não compartilham valores resolvíveis;
  proteção acumulada na mesma invocação não compartilha capturas.

Não há parâmetros em `count`, pipelines, aritmética, soma/média, comparador novo
ou contagem em strings/objetos/header. Declaração inteira e arquivo inteiro são
validados antes da rede, inclusive erro tardio em LF/CRLF. Esta ampliação remove
somente a antiga proibição de `count` em captura JSONPath.

[Diferencial Hurl 8.0.0](compatibility.md#captura-de-contagem--us-33--issue-66)
separa códigos de captura, opt-in, colisões e ordem; não afirma equivalência de
toda seleção ou de toda a linguagem. [Validação](validation.md#us-33--issue-66--história-0033).

<a id="desigualdade-json"></a>

## Desigualdade JSON — US-32 / issue #64

`jsonpath "consulta" != <literal JSON>` compara um valor **presente**, usando a
mesma igualdade tipada e exata de `==`. Tipos diferentes diferem; `1`, `1.0` e
`1e0` não diferem. Ordem de chaves não altera objetos; ordem de arrays participa.
Ausência falha com **1**, inclusive contra `null`; `null` presente é valor.
JSON inválido/percurso incompatível retorna **2**, nunca sucesso por negação.

Esperados seguem US-30: sem flag, placeholders em strings são literais; com
`--render-asserts`, o lote provisório prevalece sobre valores confirmados/iniciais
na assertion alcançada, antes da consulta/decode. Ordem US-34: status → capturas
provisórias → assertions → confirmação; erro estático impede todo envio.
`count !=`, desigualdade de header/body e negação genérica não são suportados.

[Rotação completa](../testdata/scenarios/scenario_json_inequality_rotation.httpgo):

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

Execute com `--render-asserts --secret old_token --secret new_token` e
`--var base_url=http://localhost:8080`. As flags `--secret` marcam nomes, nunca
fornecem credenciais. Diagnósticos ocultam operandos secretos/derivados e capturas
secretas pendentes; coleções são resumos de tipo, não seu conteúdo.
**Diferença de valor não garante revogação do token antigo, validade do novo nem
ausência de outros efeitos**: verifique essas propriedades com outras requisições.

<a id="operandos-dinâmicos"></a>

## Operandos dinâmicos — US-31 / issue #62

`--render-asserts` habilita placeholders **somente** em operandos de igualdade
JSONPath de assertions/capturas. Exemplo:

```hurl
GET {{base_url}}/orders
HTTP 200
[Asserts]
jsonpath "$.items[?(@.number=={{order_number}})].status" == "PLACED"
jsonpath "$.items[?(@.name=='Order {{suffix}}')].id" exists
[Captures]
selected_id: jsonpath "$.items[?(@.number=={{order_number}})].id"
```

Nas consultas de captura, `order_number`/`suffix` precisam ser iniciais ou
capturas **confirmadas anteriores**. Nas assertions também podem vir do lote
provisório válido da própria resposta (US-34).
[Exemplo completo](../testdata/scenarios/dynamic_filter.httpgo), incluindo captura
anterior e envio dependente, executado pelo E2E com HTTP real.

- Placeholder completo não delimitado conserva escalar tipado: string, número
  (`json.Number` exato), booleano ou `null`. Nunca converte string numérica em número.
- Dentro de literal de aspas simples/duplas, compõe texto pelas regras escalares
  de URL/header: texto vazio, números, booleanos e `null`; arrays/objetos não têm
  conversão implícita. Também são rejeitados como operando completo, mesmo vazios.
- O texto substituído é dado vinculado ao plano, sem expansão recursiva ou reparse
  JSONPath. Aspas, barras, colchetes ou operadores no valor não criam passos novos.
  Delimitadores/escapes da string externa do cenário e da interna continuam separados.
- Sem flag, placeholders nas consultas são erro estático **2**, mesmo em strings
  e com variável definida. É mudança explícita da permissão US-28 de literal
  semelhante a placeholder, não mudança dos esperados literais US-30.
- Com flag, toda estrutura estática é validada antes do primeiro HTTP do arquivo,
  sem resolver valores. Propriedades, índices, caminhos relativos, operadores e
  consulta inteira dinâmicos são rejeitados, sem ampliar a gramática US-28.
- Só a consulta alcançada resolve seus operandos. Na assertion, preparado primeiro
  o esperado US-30, depois **todos** os operandos antes de decode/seleção, inclusive
  se não houver itens. Ausência/tipo inválido retorna **2** localizado na consulta,
  sem publicação/avanço. Status/assertion anterior divergente mantém **1** e impede
  resolução antecipada de assertions posteriores. Erro de captura precede assertions.
- Capturas não veem valores provisórios do próprio lote. Seleção/publicação falha
  descarta o lote inteiro sem sobrescrever valores anteriores. Proteção já
  descoberta permanece. Ordem US-34: status → capturas provisórias → assertions → confirmação → callback.
- Zero/um/vários, ausência versus `null`, preservação de arrays e percurso estrito
  são os da US-27/28. Coleção capturada não vira URL/header nem primeiro ID.
- Consulta apresentada usa fonte original sanitizada e operandos ocultos; nunca
  expressão resolvida ou coleção selecionada. Proveniência secreta protege também
  derivados, categorias e valores capturados por seleção secreta. Marque segredos
  arbitrários explicitamente; sanitização não detecta toda codificação desconhecida.

Este opt-in é evolução HTTPGo, não uma opção Hurl. Hurl 8.0.0 interpola sem a
flag e aceita índice dinâmico; tipos, precisão, escapes e publicação têm
[diferenças observadas](compatibility.md#operandos-dinâmicos--us-31--issue-62).
Sem funções/scripts/lógica/novos operadores ou JSONPath completo.

<a id="seleção-estática-por-igualdade"></a>

## Seleção estática por igualdade — US-28 (issue #56)

Assertions e capturas aceitam `[?(@.caminho == literal)]`, combinável com
propriedades por ponto, índices não negativos e wildcards de arrays. Exemplo:

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

O atributo pertence ao próprio item, sem combinar entidades. Literal escalar é
string entre aspas simples/duplas, número JSON exato, booleano ou `null`.
Escapes da string interna são separados dos escapes da string externa do cenário.
Exemplos de consultas na sintaxe `.httpgo`:

```hurl
GET {{base_url}}/products
HTTP 200
[Asserts]
jsonpath "$[?(@.name=='it\\'s pizza')].id" exists
jsonpath "$[?(@.name==\"a\\\"b\")].id" exists
jsonpath "$[?(@.name=='\\uD83C\\uDF55')].id" exists
```

Dentro do literal: escapes JSON `\\`, `\/`, `\"`, `\b`, `\f`, `\n`, `\r`,
`\t`, `\uXXXX` (inclusive pares surrogate); `\'` é aceito em aspas simples.
A barra precisa ser duplicada novamente na string externa do cenário. Valores são
comparados após decode, sem avaliar delimitadores como novos passos.
Não há coerção; atributo ausente não corresponde nem a `null`. Propriedade em
não objeto, índice em não array e filtro em não array geram erro `2`, inclusive
em ramos mistos. Zero seleções é ausência, uma é o próprio valor, várias são
array ordenado, sem achatamento. Predicados recebem somente esse valor reduzido.
Captura ausente retorna `2`; `null` é válido. Coleções só podem ser reutilizadas
como valor JSON completo, nunca como texto em URL/header.

**Evolução explícita US-31 (issue #62):** placeholders em consultas, inclusive
em literais, exigem `--render-asserts`; sem a flag são rejeitados antes da rede.
Com a flag, somente operandos de igualdade admitem `{{nome}}` como valor escalar
tipado ou composição textual como `'Order {{suffix}}'`. Arrays/objetos não são
convertidos em texto nem admitidos como operando completo. O plano estrutural é
validado antes do HTTP; operandos são vinculados na avaliação, nunca concatenados
ou reinterpretados como expressão. Aspas e texto de filtro inseridos são dados,
e não há expansão recursiva. Exemplo executável:
`testdata/scenarios/dynamic_filter.httpgo` (usar `--render-asserts`).
Consultas inteiras, propriedades, índices e operadores dinâmicos continuam
rejeitados, assim como operadores além de `==`, existência, funções, scripts,
regex e operadores lógicos em filtros, antes de qualquer HTTP do arquivo. Ordem US-34: status → capturas provisórias → assertions → confirmação. Falhas
não imprimem coleções; literais dos filtros são sempre ocultados na consulta
apresentada, inclusive escapes de segredos, e o plano não entra nos relatórios.
Exemplo versionado: `testdata/scenarios/scenario_filter.httpgo`.

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

<a id="inclusão-e-ausência-jsonheader"></a>

### Inclusão e ausência JSON/header — US-23 (issue #44)

```hurl
GET {{base_url}}/validation
HTTP 200
[Asserts]
jsonpath "$.codes" contains "REQUIRED"
jsonpath "$.codes" not contains "OTHER_TENANT"
jsonpath "$.passwordHash" not exists
header "Set-Cookie" not contains "Domain"
```

JSON `contains` e `not contains` aceitam esperado escalar literal (string,
número, booleano ou `null`). Em strings, somente esperado string é aplicável:
busca substring literal, distinguindo caixa, sem normalização de espaços ou
Unicode. Arrays comparam elementos diretos por tipo e valor, com igualdade
numérica exata, sem busca recursiva nem comparação de JSON textual.
Array vazio não contém nenhum escalar; toda string contém `""`.
Ausência aprova `not contains` e `not exists`; `null` presente não é ausência.
Número, booleano, objeto ou `null` selecionados, ou string com esperado não
string, falham com `1` nos dois sentidos de pertencimento.
`not exists` não aceita operando e falha para qualquer valor presente,
inclusive raiz, vazio, false e zero. Índice fora do array é ausência.
JSON inválido e percurso incompatível (inclusive através de `null`) continuam
sendo erros `2`, nunca sucesso por negação.

Header `not contains` exige string literal e aprova ausência. Consulta ignora
caixa do nome e usa somente a primeira ocorrência, sem concatenar ou dividir
por vírgula. Valor presente vazio contém `""`, portanto sua negação falha.
Consultas e esperados são estáticos: placeholders dentro de strings permanecem
literais. Não há `header not exists`, negação genérica, operadores lógicos,
`count contains` ou filtros além da igualdade estática US-28. Projeção `[*]` de arrays é o recorte US-27 abaixo. Erro estático tardio impede todos
os envios. Desde a US-34, status antecede todo o lote de capturas, seguido de
assertions na ordem escrita e confirmação; primeira falha interrompe o arquivo. Diagnósticos localizados mostram somente
presença/tipo e esperado escalar protegido, nunca a coleção selecionada.
Headers sensíveis e captura secreta pendente permanecem ocultos.
Exemplo versionado: `testdata/scenarios/scenario_assert_membership.httpgo`.
Os 37 pares `membership_*` e diferenças de headers repetidos, literais,
expoentes e percurso estão em [compatibilidade](compatibility.md#inclusão-e-ausência--us-23--issue-44).

<a id="busca-textual-no-body"></a>

### Busca textual no body — US-21 (issue #40)

```hurl
GET {{base_url}}/export
HTTP 200
[Asserts]
body contains "customer"
body not contains "passwordHash"
body not contains "trackingToken"
```

`body contains "trecho"` e `body not contains "trecho"` consultam o texto
completo da resposta final, consumida e fechada, sem reserializar JSON.
Exigem `HTTP` na entrada e uma única string literal; placeholders não interpolam.
A busca diferencia caixa e não normaliza espaços, quebras de linha ou Unicode.
Todo texto contém `""`, inclusive body vazio: sua negação sempre falha.
Charset vem do primeiro `Content-Type`; sem charset usa UTF-8 estrito.
Rótulos reconhecidos usam o índice de encodings web (incluindo ISO-8859-1).
Charset desconhecido ou conversão inválida gera erro `2`, também na negação.
Status → capturas provisórias → assertions na ordem escrita → confirmação (US-34).
A primeira falha impede confirmação e próximas requisições. Condição falsa retorna `1`, sucesso `0`.
Diagnósticos localizados identificam operador e esperado protegido, nunca body
ou fragmentos recebidos. Captura secreta provisória oculta o esperado sem
confirmar seus valores. Busca literal não detecta todas as codificações de dados sensíveis.
`body matches`, filtros e outros predicados de body não são suportados;
a query de extração `regex` em assertions segue US-36 acima.
Interpolação de esperados só com `--render-asserts` (US-30). Extração regex de body em `[Captures]` é o recorte US-25.
Os 19 pares `body_*` confrontados com Hurl 8.0.0 são publicados em
[compatibilidade](compatibility.md#ampliação-us-21--busca-textual-no-body-issue-40).
ISO-8859-1 resolve para windows-1252 (incluindo byte 0x80 → €), conforme esse
corpus. Rótulos não confrontados não têm equivalência geral anunciada.
Aceitar sintaxe não implica equivalência integral ao Hurl.

<a id="upload-multipart-básico"></a>

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
origem, TLS verificado e prazo único configurável por tentativa (default de dez
segundos), sem retry implícito. Polling opt-in segue a US-26 abaixo. Resposta 413
completa é avaliada normalmente, inclusive antecipada; erro de conexão sem resposta
completa é transporte (2). Leituras HTTP terminam antes de rebobinar/fechar a fonte;
falhas de leitura/fechamento não viram sucesso. I/O local bloqueado pelo SO continua
sujeito à limitação descrita na US-09.

**Mudança de contrato:** `[MultipartFormData]` antes rejeitado agora é suportado
neste recorte. O antigo negativo global virou o caso de multipart em GET;
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
`testdata/scenarios/`, nos arquivos com prefixos `unsupported_` e `invalid_`.

<a id="body-de-arquivo-externo"></a>

### Body de arquivo externo — US-09 (issue #22)

Alternativamente ao JSON inline, `POST`, `PUT`, `PATCH` e `DELETE` aceitam
`file,<caminho>;` como body completo. O caminho é literal entre `file,` e o
`;` final, incluindo espaços, aspas e barras, sem escapes nem interpolação.
Vírgula e ponto e vírgula no nome não são aceitos. GET continua sem body;
body duplicado/misturado ou header após body rejeita todo o cenário.
Caminhos relativos partem do diretório do cenário, não do diretório corrente;
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

<a id="extensão-httpgo-json-externo-com---render-files"></a>

## Extensão HTTPGo: JSON externo com --render-files — US-10 (issue #25)

Esta extensão é **separada do subconjunto compatível com Hurl**. Não se promete
renderização externa equivalente pelo Hurl. Ative por execução, antes ou depois
do arquivo após `test` (flag sem valor; `--render-files=false` não é aceito):

```sh
httpgo test tests/products.httpgo --render-files \
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
inserido. Chaves e caminhos não interpolam; sem novas expressões.
Variáveis iniciais (CLI/YAML) e capturas anteriores usam a mesma sessão.
JSON estático válido conserva todos os bytes, inclusive whitespace externo LF/CRLF.

O parser valida o cenário inteiro antes da rede **sem abrir payloads**. Ler,
validar e renderizar o conteúdo externo ocorre só na entrada corrente, antes do
executor e depois de fechar o arquivo. Exige um único valor JSON completo UTF-8.
Vazio, somente whitespace, não JSON, conteúdo adicional, template malformado,
variável ausente, tipo incompatível ou JSON final inválido produz **erro de
renderização (2)**. A entrada afetada e seguintes não são enviadas nem contadas
como iniciadas; resultados anteriores permanecem. Estrutura inválida do cenário
continua rejeitando todos os envios. Abertura/leitura/fechamento mantêm FileError
com posição da referência no cenário, categoria distinta de conteúdo/renderização.

Diagnósticos identificam payload e causa controlada, com linha/coluna em bytes do
conteúdo **original** (LF/CRLF), nunca do JSON expandido. Sem offset confiável,
usam 1:1; incluem a referência no cenário. Nomes são sanitizados e exibidos com quoting,
sem despejar caminho absoluto operacional calculado, causas brutas ou payload.
Resultados/callbacks não retêm corpos, planos ou referências operacionais.
Segredos marcados, capturas secretas e derivados/escapes conhecidos permanecem
protegidos em sucesso/falha, sem descobrir segredos arbitrários do arquivo.
Permanecem os limites de memória, acesso local sem sandbox, contexto e I/O bloqueado
já descritos; nenhuma nova goroutine/retry/limite de tamanho é introduzida.

<a id="extensão-httpgo-configuração-yaml"></a>

## Extensão HTTPGo: configuração YAML — US-11 (issue #27)

`httpgo test products.httpgo --config projeto.yaml` seleciona somente o arquivo
indicado, inclusive fora do cwd. A flag aceita `--config=projeto.yaml`, antes ou
depois do cenário após `test`; vazia, sem argumento ou repetida é uso inválido.
Sem a flag, carrega somente `.httpgo.yaml` do **diretório corrente**, sem buscar
ancestrais. Ausência do arquivo opcional equivale a defaults; inexistência
explícita ou erro de abertura/leitura/fechamento é configuração inválida (2).
Arquivo vazio ou com somente espaços/comentários equivale a configuração ausente.

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

```hurl
POST {{api.baseUrl}}/products
Content-Type: application/json
{"price": {{product.price}}}
HTTP 201
```

Execute `httpgo test products.httpgo`, ou use os exemplos separados em
`testdata/config/project.yaml` e `testdata/scenarios/scenario_config_products.httpgo`
com `--config testdata/config/project.yaml`. Para trocar somente o destino,
acrescente `--var api.baseUrl=http://localhost:9090` (valor não sigiloso).

### Schema e conversão

- No máximo **1 MiB (1.048.576 bytes)**, verificado durante a leitura, inclusive
  para espaços/comentários. Arquivo exatamente no limite é permitido.
- Um documento YAML apenas, raiz mapa, `version: 1` obrigatória (inteiro).
  Opcionais: `defaults` (mapa com somente `timeout`), `variables` (mapa) e
  `secrets` (lista de nomes válidos). Campos desconhecidos, chaves repetidas e
  estruturas/tipos incompatíveis são erro, nunca ignorados.
- Âncoras, aliases e tags explícitas são proibidos em qualquer profundidade,
  inclusive tags padrão como `!!str`. Não há expansão ou resolução externa.
- Mapas em `variables` viram caminhos hierárquicos exatos conforme nomes ASCII
  das variáveis. `api: {baseUrl: ...}` define `api.baseUrl`, **não** `api`.
  Chaves pontuadas também são aceitas, mas caminhos repetidos/sobrepostos e
  conflitos valor/descendente são rejeitados, em qualquer ordem.
- Valores: strings, números finitos, booleanos, null ou arrays JSON-compatíveis.
  Mapas **dentro de arrays** são objetos de conteúdo com chaves string, nunca
  caminhos; duplicações e tipos incompatíveis também são rejeitados ali.
  Datas/timestamps não são valores JSON: use aspas para torná-los strings.
  Strings numéricas entre aspas mantêm tipo string. Números usam a representação
  original/canônica sem passar seus valores por float64; precisão inteira,
  decimal e aninhada não é arredondada. `.inf`/`.nan` são rejeitados.
- Referências explícitas `${NOME}` seguem a extensão US-13 abaixo: somente folhas
  string completas de `variables`, nunca listas ou objetos nelas contidos.
  Sintaxe inválida/não suportada é rejeitada por fonte, mesmo com override.
  Não há overrides automáticos `HTTPGO_*`, stdin ou hooks. Ambientes: US-12 abaixo.

### Composição, prazo e proteção

Precedência efetivamente implementada: **defaults internos < YAML base < ambiente selecionado < CLI**.
A fonte CLI é validada separadamente: repetir `--var` para o mesmo nome continua
erro. Entre fontes, CLI substitui o mesmo caminho ou acrescenta outro, sem perder
irmãos. Escalares, listas e null substituem integralmente, sem combinar listas.
Objetos CLI permanecem valores opacos, sem propriedades acessíveis; não podem
substituir um mapa hierárquico deixando descendentes conflitantes.

`defaults.timeout` é **número de segundos entre 0.001 e 86400**, inclusive,
representável exatamente em milissegundos inteiros. Não admite texto com unidade,
string numérica, não finitos ou arredondamento. Omissão conserva dez segundos.
O prazo é por requisição lógica, incluindo conexão, TLS, redirects e consumo/
fechamento da resposta, sem tetos internos de dez segundos. Cada nova requisição
recebe orçamento próprio; contexto cancelado ou prazo menor do chamador prevalece.
Não há retry implícito, interrupção prometida de I/O local bloqueado ou mutação
do executor; polling explícito por entrada segue a US-26 abaixo.

`secrets` acumula com `--secret`, sem exigir variável existente. Pode proteger
captura futura; usar valor ainda ausente continua falhando. Override nunca remove
proteção, inclusive com lista posterior vazia. Valores conhecidos da base,
ambiente e substitutos CLI (mesmo os sobrescritos), inclusive variantes e
constituintes JSON, são protegidos desde a inicialização dos writers/contexto;
a sessão mantém a proteção dinâmica das capturas/derivados e callbacks.
Credenciais reais em flags seguem proibidas. YAML não protege o conteúdo em disco:
use permissões `0600`; não há descoberta universal de segredos arbitrários.

Carga/validação precedem parsing do cenário e qualquer HTTP. Erro retorna **2**,
sem aplicação parcial, com arquivo/linha/coluna quando identificáveis (fallback
1:1), mensagens controladas, sem ecoar nomes arbitrários, trechos ou erros brutos
YAML/I/O. Contexto é checado antes/depois da carga e antes do envio. Origem exibida
passa por quoting e proteção; o caminho selecionado não redefine `OriginDir` do
cenário. Contexto, marcas e prazo são locais à invocação, sem vazamento na próxima.

Esta configuração é **extensão HTTPGo**, não parte do formato Hurl. Os testes YAML
são separados do corpus diferencial Hurl; nenhuma flag própria é passada ao Hurl.

<a id="extensão-httpgo-ambientes-nomeados"></a>

## Extensão HTTPGo: ambientes nomeados — US-12 (issue #29)

```sh
httpgo test tests/products.httpgo --env local
httpgo test tests/products.httpgo --config projeto/base.yaml --env=staging
```

`--env` seleciona no máximo um ambiente por invocação, antes/depois do cenário
após `test`. Repetição, mesmo com nome idêntico, nome vazio ou argumento ausente
é erro 2. Nomes aceitam somente `[A-Za-z0-9_-]+`; pontos, espaços, Unicode e
separadores de caminho não são aceitos. O nome é validado antes de compor o caminho.
Sem seleção, nenhum arquivo de ambiente é aberto.

O único caminho é `environments/<nome>.yaml`, relativo ao diretório da configuração
base selecionada (explícita ou descoberta), inclusive se vazia. Sem configuração
base, parte do diretório corrente. Não busca ancestrais, arquivos alternativos ou
ambientes relativos ao cenário. Ausente/ilegível, falha de leitura/fechamento ou
cancelamento é erro antes de qualquer HTTP, sem fallback ou configuração parcial.
Não altera cwd, `Scenario.OriginDir` ou resolução dos payloads.

Ambiente usa o schema e proteções da base, exceto `version: 1` que é **opcional**.
Somente `defaults.timeout`, `variables` e `secrets`; arquivo vazio/whitespace/só
comentários não acrescenta overrides. Limite de **1 MiB por arquivo**, documento
único, chaves únicas, mesmos tipos/nomes e timeout exato entre 1 ms e 24 horas.
Versão diferente, campo desconhecido e recursos YAML proibidos são rejeitados
com arquivo/posição e mensagem segura antes de qualquer envio. Referências
`${NOME}` seguem a extensão US-13 abaixo, com validação independente por fonte.

Base → ambiente → CLI preserva folhas irmãs, strings numéricas e precisão dos
números. Mapas são caminhos e combinam profundamente; escalares, listas e `null`
substituem a folha inteira, sem concatenar listas ou tratar `null` como ausência.
Um mapa, inclusive vazio, não pode virar valor, nem um valor virar mapa com
descendentes. Cada fonte é validada antes do merge: override não esconde erro.
Objetos CLI continuam opacos e duplicação de `--var` continua erro.
`secrets` é união das três fontes, mesmo com lista vazia posterior; conserva
proteção dos valores anteriores/substitutos, variantes, capturas e derivados,
inclusive em erros antes da sessão. Marcação isolada não cria valor. Cada
invocação tem configuração/contexto/capturas novos, sem estado compartilhado.

Exemplo completo de base, ambiente e CLI: [guia de configuração](configuration.md).
Fixtures adicionais: `testdata/config/environment_base.yaml`,
`testdata/config/environments/{local,staging}.yaml` e
`testdata/scenarios/scenario_environment_products.httpgo`.
Sem múltiplos ambientes, seleção automática, stdin ou mudanças nos caminhos de
payloads. Referências de processo: US-13 abaixo. Não é uma capacidade Hurl;
testes separados do corpus diferencial, sem fornecer `--env` à outra ferramenta.

<a id="extensão-httpgo-referências-ao-ambiente-do-processo"></a>

## Extensão HTTPGo: referências ao ambiente do processo — US-13 (issue #32)

```yaml
# Configuração base: version: 1 obrigatória; no ambiente selecionado é opcional.
version: 1
variables:
  auth:
    token: ${OWNER_TOKEN}
```

```hurl
GET http://localhost:8080/profile
Authorization: Bearer {{auth.token}}
HTTP 200
```

Com `OWNER_TOKEN` fornecido externamente, `httpgo test tests/profile.httpgo --config base.yaml`
entrega o header e retorna 0 para status 200. Em `environments/local.yaml`, omita
`version` se desejar e use `httpgo test tests/profile.httpgo --env local`.
Exemplos versionados: `testdata/config/process_base.yaml`,
`testdata/config/environments/process.yaml`, `testdata/scenarios/scenario_process_profile.httpgo`.

**Sintaxe e validação por fonte:** aceita somente `${NOME}` ocupando todo o valor
string de uma folha em `variables`. O nome é ASCII `[A-Za-z_][A-Za-z0-9_]*`, sem
espaços ou pontos. Mapas continuam sendo caminhos hierárquicos. Listas e quaisquer
objetos/listas internos não admitem referências. Composição parcial, múltiplas
referências, nomes inválidos ou delimitadores malformados falham antes de HTTP,
mesmo se ambiente/CLI substituir a declaração. Outros campos YAML não resolvem
referências. Não é linguagem de templates nem sintaxe do formato Hurl.

**Ordem e tipos:** validar cada fonte, compor base < ambiente < CLI e consultar
apenas referências ainda efetivas. Uma referência descartada pode estar ausente;
não é consultada. Todas as efetivas são obrigatórias, inclusive sem uso no cenário.
Ausência gera 2 antes de parsing/HTTP, identificando referência, chave de configuração
e arquivo/linha/coluna quando disponíveis. Não publica contexto parcialmente resolvido.
Consulta distingue ausência de presença vazia. Vazio é string vazia, podendo falhar
em contexto incompatível (como URL), não ausência nem substituição global de texto
vazio na saída. Sempre serializa como **string JSON**: `123`, `true`, `null` e
`{"x":1}` permanecem strings. Conteúdo `${OUTRA}`/`{{outra}}` nunca é reinterpretado.
Não importa variáveis não referenciadas nem introduz opções `HTTPGO_*`.

**Proteção:** cada destino é marcado antes dos overrides; a união com `secrets` e
`--secret` não é removida por substituição. `--var` pode fornecer substituto
fictício/não sigiloso, mantendo precedência e marca; credenciais reais em argumentos
seguem proibidas. Só os valores efetivos referenciados são consultados/conhecidos;
não se consultam segredos descartados só para proteger sua grafia desconhecida.
Valores conhecidos/substitutos/variantes são registrados antes de diagnósticos,
inclusive se outra referência estiver ausente ou houver erro de argumento/composição/
parsing antes da primeira sessão. A comparação não muda; operandos de assertions,
resultados/callbacks, stdout/stderr são sanitizados, inclusive diante de eco em
header comum/JSON. Sessão continua protegendo capturas e derivados em URL, headers,
JSON inline/externo opt-in e campos multipart, sem alterar bytes de arquivo.
A US-17 reutiliza a mesma política no relatório JUnit.

Cada invocação lê seu próprio ambiente, sem cache global, dotenv ou herança de
valores/marcas/capturas da execução anterior. Cancelamento é verificado na preparação
e antes do envio, sem novo timeout de resolução nem promessa de interromper I/O
local bloqueado. Falha de saída/short write continua prioritária (2), sem retry ou
próximo envio. Não há stdin, cofres, composição parcial, referências em listas ou
identificação garantida de segredos desconhecidos/transformações arbitrárias.

<a id="variáveis-tipadas-e-templates"></a>

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
ordem e tipo impede todos os envios. Duplicações são erros na mesma fonte;
CLI pode sobrescrever explicitamente o mesmo nome YAML, conforme a US-11.
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
httpgo test products.httpgo --var api.baseUrl=http://localhost:8080 \
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
valores não sigilosos/fictícios são adequados aqui. A fonte pelo ambiente do
processo usa referências explícitas `${NOME}` na extensão US-13 acima; não há
entrada de segredos por stdin.
Configuração YAML é a extensão US-11 descrita acima. Capturas de respostas não são uma nova fonte de
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
httpgo test products.httpgo --var api.baseUrl=http://localhost:8080 \
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

<a id="capturas-literais-de-headers"></a>

## Capturas literais de headers — US-22 (issue #42)

```hurl
GET {{base_url}}/catalog
HTTP 200
[Captures]
catalog_etag: header "ETag"

GET {{base_url}}/catalog
If-None-Match: {{catalog_etag}}
HTTP 304
```

`nome: header "nome-do-header"` consulta somente a resposta final completa,
sem distinguir caixa do nome. Seleciona **a primeira ocorrência**, inclusive
vazia; não junta ocorrências nem divide vírgulas. Esta política preserva a
US-07 e é uma diferença deliberada frente a headers multivalorados do Hurl.
O valor é sempre string literal: aspas de ETag, números, JSON e placeholders
recebidos são preservados, sem coerção ou interpolação recursiva. Não exige
body JSON nem retenção do body quando nenhuma outra consulta precisar dele;
a resposta ainda é consumida e fechada integralmente antes das validações.

Header ausente é erro de captura (2). Status antecede capturas; assertions usam o lote provisório;
lotes mistos de header/JSONPath são atômicos e mantêm as mesmas regras de nomes,
isolamento e colisões abaixo. Sintaxe inválida tardia impede todos os envios.
Índices e todas as ocorrências não são suportados. O único filtro de captura de header suportado é `regex`, descrito na US-25 abaixo.

`Authorization`, `Proxy-Authorization`, `Cookie` e `Set-Cookie` são protegidos
sem distinguir caixa, incluindo a proveniência do nome capturado em composição
e escapes conhecidos. Marcações explícitas e proteção dinâmica acumulam na
invocação, inclusive após descarte/renovação e troca de arquivo (US-34), sem
compartilhar valores resolvíveis entre arquivos. Diagnósticos não imprimem coleções de headers nem bodies.
Valor vazio não provoca substituição global; não há detecção de codificações
arbitrárias. Capturar `Set-Cookie` **não** cria `Cookie`, não extrai atributos e
não ativa cookie jar; extração de valor de cookie pertence à US-25.
Execute `httpgo test cache.httpgo --var base_url=http://localhost:8080` contra API
com cache condicional; exemplo versionado em `testdata/scenarios/scenario_capture_cache.httpgo`.
A [matriz diferencial](compatibility.md#capturas-literais-de-headers--us-22--issue-42)
registra as diferenças observadas de repetição, interpolação JSON, ordem e colisões
com Hurl 8.0.0; não há promessa de equivalência geral.

<a id="extração-regex-de-body-e-header"></a>

## Extração regex de body e header — US-25 (issue #48)

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

Marque `otp_code` com `--secret otp_code` ou `secrets` na configuração.
Para compor cookie explicitamente, sem atributos de resposta nem cookie jar:

```hurl
GET {{base_url}}/session
HTTP 200
[Captures]
refresh_token: header "Set-Cookie" regex "refresh=([^;]+)"

GET {{base_url}}/next
Cookie: refresh={{refresh_token}}
HTTP 200
```

O padrão é uma string JSON estática, usando a sintaxe e os limites de regex Go
da US-15, nunca interpolada. Extrai **somente o primeiro grupo da primeira
correspondência**, como string, sem coerção ou expansão recursiva. Grupo vazio
participante publica `""`; ausência de correspondência, falta de grupo ou primeiro
grupo não participante são erros de captura (2). Padrão/escape inválido ou regex
não suportada rejeita o arquivo inteiro antes da rede; regex válida sem grupo
só falha na etapa de captura.

Body usa a decodificação estrita da US-21 (UTF-8 por padrão, charset declarado,
sem normalização); header seleciona somente a primeira ocorrência da US-22, sem
decodificar body nem exigir JSON. Ausência de header e falha textual geram 2.
Desde a US-34: status → capturas provisórias → assertions → confirmação atômica, com renovação exata.
Capturas de headers sensíveis protegem também o grupo extraído imediatamente,
mesmo se outra captura falhar; dados secretos do body exigem marcação explícita.
Resultados não conservam padrões, expressões compiladas ou texto da resposta.
Escolha de grupo, substituição, regex dinâmica e pipelines arbitrários continuam
fora do recorte. Assertions podem usar captura própria válida desde a US-34.

<a id="capturas-tipadas"></a>

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
na entrada ou vazia. Cada declaração JSON tem a forma `nome: jsonpath "consulta"`;
a US-22 também aceita `nome: header "nome-do-header"`, descrita acima.
Comentários em linha própria e LF/CRLF mantêm a associação. A próxima request-line
encerra a seção; `[Asserts]` também pode sucedê-la na mesma entrada. Outras queries
e filtros de captura não são aceitos nem ignorados.
Nomes seguem a mesma regra ASCII pontuada das variáveis.

O subconjunto de JSONPath aceita `$`, propriedades por ponto, índices decimais
não negativos e wildcard de array `[*]`, combináveis: `$.items[0].token` e
`$.categories[*].products[*].id`. Propriedades simples usam letras ASCII, dígitos
e `_`; nomes com outros caracteres exigiriam sintaxe ainda não suportada.
Filtros estáticos por igualdade são o recorte US-28 acima. Não há wildcard de objeto (`.*`), outros filtros, descida recursiva, slices, união,
propriedades entre colchetes ou índices negativos.

**Ampliação intencional US-27**, compartilhada por assertions e capturas:
`[*]` expande somente arrays em ordem de percurso. Ao terminar a consulta,
zero resultados significa ausência; um devolve o próprio valor; vários devolvem
um array ordenado (inclusive repetições). Atributos ausentes e índices fora do
array eliminam somente aquele ramo; `null` selecionado permanece presente.
Selecionar um único array preserva esse array, inclusive vazio; selecionar vários
arrays não os achata. `count` conta o array reduzido, não correspondências internas:
um número único ou ausência falha com 1, dois números contam 2.
Propriedade em não objeto, índice/wildcard em não array gera erro 2 na seleção
expansiva inteira, mesmo ao lado de ramos válidos. Captura sem resultado falha
com 2; publicação tipada continua atômica. Diagnóstico expansivo resume presença,
tipo e contagem, sem despejar seleção ou esperado composto; resumos protegidos
ficam ocultos. Caminhos simples conservam seus contratos e diagnósticos.

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

Para `{"issues":[{"code":"REQUIRED"},{"code":"INVALID"}]}`, a demonstração
passa independentemente da ordem. `/echo` deve aceitar o array JSON com status 204.
Sem REQUIRED ou com OTHER_TENANT, falha com 1, sem captura/próximo envio.
Exemplo: `testdata/scenarios/scenario_projection.httpgo`; execute com
`httpgo test testdata/scenarios/scenario_projection.httpgo --var base_url=http://localhost:8080`.
Consultas inválidas/não suportadas e nomes inválidos em entrada tardia rejeitam
**todo** o arquivo, com arquivo/linha/coluna originais antes de qualquer rede.

**Preservação US-27b / issue #54:** a adoção interna de OjG `jp` é limitada à
navegação das projeções já aceitas. Caminhos simples permanecem nativos e não há
flag de avaliador, fallback ou nova sintaxe. Números seguem tipados/exatos, inclusive
inteiros além de `2^53`/`int64`, decimais próximos e expoentes extremos; grafias
capturadas são preservadas, sem float, coerção textual, infinito ou underflow.
Igualdade numérica mantém formas equivalentes como `1.00` e `1e0`.

O [exemplo de precisão](../testdata/scenarios/scenario_projection_precision.httpgo)
seleciona `9007199254740992` e `9007199254740993` por caminhos simples, conta a
projeção e reutiliza a captura como `[9007199254740992,9007199254740993]` em POST.
Com os IDs distintos termina em 0; substituir o segundo pelo primeiro falha em 1,
sem publicação nem POST. Use
`httpgo test testdata/scenarios/scenario_projection_precision.httpgo --var base_url=http://localhost:8080`
contra `/products` e `/echo` do contrato sintético do exemplo. Isso não declara
suporte a filtros por consequência da biblioteca (igualdade estática é a US-28), funções nem RFC integral; cardinalidade, ausência/null, erros
estritos, atomicidade e sigilo acima continuam inalterados. Ver
[compatibilidade](compatibility.md#preservação-de-jsonpath--us-27b--issue-54).

Para capturas JSONPath, a resposta final deve conter exatamente um valor JSON válido. **Ampliação
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

Exemplo mínimo (`testdata/scenarios/scenario_capture_products.httpgo`):

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

Execute `httpgo test products.httpgo --var base_url=http://localhost:8080` contra
API que devolva `{"id":9007199254740993}`. O ID chega exato no path e como número
no body seguinte; os três status são validados. O cenário autenticado completo
está em `testdata/scenarios/scenario_capture_auth_products.httpgo` e é exercitado pelos E2E.

Após consumir/fechar a resposta, status aceito permite extrair todo o lote antes
de assertions, independentemente da ordem das seções (US-34). Captura inválida
retorna 2 antes de assertions. Entre assertions, vale a ordem escrita: header
divergente anterior ao JSON inválido retorna 1; invertendo, avaliação JSON retorna 2.
Falhas não confirmam lote nem avançam. 4xx/5xx esperado permite capturar;
sem expectativa, não se presume validação de status.

Capturas têm escopo separado das variáveis iniciais e uma sessão nova por execução.
Sobrescrita exata de inicial/captura anterior é permitida (US-34). Duplicações
no lote e conflitos ancestral/descendente continuam falhando, independentemente
do tipo. O lote provisório só fica disponível às assertions se todas as consultas,
valores e nomes passarem; confirmação exige aprovação e contexto ativo. Colisão, captura inválida ou uso antecipado/ausente
retorna 2 e interrompe as entradas seguintes. Resultados anteriores são preservados.
Falha de captura mantém a resposta em `concluídas` e o status aprovado em
`validadas` (ou `sem_validação` sem expectativa/com `HTTP *`), sem classificá-la como transporte.

`--secret nome` pode marcar a captura antes de ela existir, sem fornecer valor.
Um valor secreto descoberto, de qualquer tipo, é protegido antes de apresentar
o resultado, inclusive se outra declaração da resposta falhar. Constituintes de
arrays/objetos também são protegidos nas formas conhecidas, sem virarem variáveis
nem alterar dados enviados. A proteção dinâmica acumula na
invocação, combina-se com a proteção inicial e cobre variantes JSON/Go/URL
conhecidas e nomes de arquivo em diagnósticos. Vazio não causa substituição global.
Bodies e valores capturados nunca entram no relatório/diagnóstico; valores
selecionados para assertions podem aparecer somente sanitizados conforme abaixo. Nada permite
identificar universalmente segredos ainda desconhecidos ou recodificações arbitrárias.

<a id="assertions-de-headers"></a>

## Assertions de headers — US-07 / issue #18

**Ampliação intencional do contrato:** `[Asserts]`, antes rejeitada globalmente,
passou a aceitar as três formas de header abaixo; exige `HTTP <status>` ou `HTTP *` na mesma
entrada. A seção não pode ser órfã, vazia ou duplicada. Pode aparecer antes/depois
de `[Captures]`; a ordem vigente é status → todas as capturas provisórias →
assertions na ordem escrita → confirmação atômica (US-34).

```hurl
GET http://localhost:8080/health
HTTP 200
[Asserts]
header "Content-Type" contains "application/json"
header "X-Request-Id" exists
header "X-Request-Id" == "req-123"
```

Execute `httpgo test health.httpgo` contra uma API que devolva
`Content-Type: application/json; charset=utf-8` e `X-Request-Id: req-123`.
O exemplo está em `testdata/scenarios/scenario_assert_headers.httpgo` e é exercitado
pelo binário nos E2E. Trocar `HTTP 200` por `HTTP *` dispensa somente a comparação
de status; assertions e capturas continuam obrigatórias quando declaradas.

- Nomes de header são tokens HTTP estáticos entre aspas duplas, consultados sem
  distinguir caixa. `==` compara a string exata; `contains` procura substring
  literal; valores distinguem caixa e não recebem normalização adicional além
  daquela feita pelo transporte HTTP (por exemplo whitespace externo do protocolo).
- Valores esperados são strings literais entre aspas duplas. `{{nome}}` é texto
  literal, com ou sem variável/captura definida; não passa pelo renderer.
  `exists` não aceita valor esperado. A US-23 acrescenta `not contains` (seção acima).
  A US-24 acrescenta `matches` com padrão string JSON (seção abaixo).
  Filtros e outros operadores continuam fora do recorte. Capturas literais são suportadas
  separadamente em `[Captures]` (US-22). Construções fora desse recorte falham
  explicitamente com posição, antes de qualquer envio do arquivo.
- Havendo ocorrências repetidas, somente a **primeira na ordem recebida** participa
  de `==`/`contains`/`not contains`/`matches`, inclusive se nomes chegam com caixas diferentes. Nunca há
  concatenação ou divisão por vírgulas: `first,second` é uma única string.
- Ausência faz `==`, `contains` e `exists` falharem; aprova `not contains`.
  Presente vazio satisfaz `exists` e `== ""`; `contains ""` passa para qualquer
  valor presente, inclusive vazio; `not contains ""` falha.
- A primeira assertion divergente interrompe a sequência com código 1, descarta
  o lote provisório e mantém os resultados anteriores. Captura inválida é erro 2
  antes de qualquer assertion. Um status numérico divergente impede ambas as etapas. Falha de transporte, consumo/fechamento do
  body ou escrita continua retornando 2, nunca aprovação parcial pelos headers.
- O diagnóstico localiza a declaração, identifica header/operador e esperado/
  recebido. `ausente` difere de `""`. Strings comuns usam quoting seguro; valores
  de `Authorization`, `Proxy-Authorization`, `Cookie` e `Set-Cookie` são ocultos
  integralmente nos dois lados, independentemente da caixa ou origem literal.
  Segredos conhecidos também são removidos dos headers comuns antes do callback
  e da escrita. Nenhuma coleção de headers ou body bruto fica no relatório.

Compatibilidade comprovada é limitada aos pares `headers_*` em `compatibility.md`.
**Diferenças deliberadas em relação ao Hurl 8.0.0:** primeira ocorrência em vez de
lista em headers repetidos; esperado literal sem `--render-asserts`. Desde a
US-34 ambos avaliam capturas antes de assertions nesses pares; transação/polling
HTTPGo têm diferenças próprias registradas na matriz US-34.

<a id="regex-de-headers"></a>

## Regex de headers — US-24 / issue #46

**Ampliação intencional:** `[Asserts]` aceita `header "nome" matches "padrão"`,
com `HTTP <status>` ou `HTTP *` na mesma entrada. Exemplo `session.httpgo`:

```hurl
GET {{base_url}}/session

HTTP 200
[Asserts]
header "Set-Cookie" matches "^session=[^;]+; Path=/; HttpOnly; Secure; SameSite=Lax$"
```

Execute `httpgo test session.httpgo --var base_url=http://localhost:8080` contra
uma API que devolva `session=<valor variável>; Path=/; HttpOnly; Secure; SameSite=Lax`.
O arquivo `testdata/scenarios/scenario_assert_header_matches.httpgo` é exercitado
pelo binário nos E2E. Valores de sessão diferentes passam; ausência de `Secure`,
formato divergente ou header ausente falham com **1**, sem captura/próximo envio.

- Padrão é **uma única string JSON entre aspas duplas**, decodificada uma vez e
  compilada durante o parse integral do arquivo, com a mesma gramática da US-15.
  `"\\s"` entrega `\s` à regex; aspas e barras usam escapes JSON. `"\x41"` não é
  válido na camada JSON. Não há delimitadores `/.../` nem operando adicional.
  A gramática dos literais dos operadores anteriores de header não mudou.
- Correspondência por trecho, distinguindo caixa por padrão. Âncoras `^`/`$`,
  classes, grupos, alternativas, repetições, Unicode e opções internas Go como
  `(?i)`, `(?m)`, `(?s)` seguem `regexp`, sem normalização Unicode. As opções não
  relaxam a validação de valores HTTP. Lookaround/backreferences são não suportados.
- Nome é estático e ignora caixa. Somente a **primeira ocorrência recebida** é
  avaliada, inclusive vazia; nenhuma ocorrência posterior corrige ou invalida o
  resultado. Não há concatenação ou divisão por vírgulas. Header presente vazio
  compara como `""`. Padrão vazio passa para qualquer valor presente; `^$` passa
  só no vazio; ausência falha mesmo com padrão vazio e é diagnosticada como ausente.
- Regex malformada, escape inválido e construção fora do recorte retornam **2**
  antes de qualquer HTTP, inclusive em entrada tardia; erro localizado não ecoa
  padrão, linha original nem erro bruto de biblioteca. Sintaxe inválida é distinta
  de construção não suportada. Negativos pequenos estão em `testdata/scenarios/`.
- Só a resposta final completa, consumida e fechada é comparada. Ordem: status →
  capturas provisórias → assertions na ordem escrita → confirmação → apresentação.
  Primeira falha impede confirmação e próximos envios, inclusive com `HTTP *`. Header anterior a
  JSON inválido pode falhar com 1; JSON anterior falha com 2. Header isolado não
  retém body nem tenta decodificar JSON. Falhas de consumo/fechamento, prazo,
  cancelamento ou escrita permanecem 2, sem retry desses erros ou aprovação parcial.
  Divergência de assertion pode usar o polling explícito da US-26.
- Padrão e real de Authorization, Proxy-Authorization, Cookie e Set-Cookie ficam
  **integralmente ocultos**, independentemente de marca explícita. Headers comuns
  aplicam proteção de segredos conhecidos/derivados; captura secreta pendente na
  mesma entrada oculta os dois operandos mesmo antes da confirmação.
  Nome, operador e posição seguros permanecem; não há coleção de headers/body.
  Clones seguros dos resultados/callbacks não conservam regex compilada, que
  retém sua fonte, mesmo em sucesso ou erro. O cenário original não é mutado.
- Placeholders `{{nome}}` permanecem **literais e estáticos**, definidos ou não;
  continuam sujeitos à sintaxe regex, não são automaticamente escapados e não
  passam pelo renderer no perfil literal. O opt-in `--render-asserts` da US-30
  abaixo habilita resolução tardia apenas do esperado.
  Query variable, regex de header como query de assertion e todas as ocorrências
  ficam fora do suporte. Extração regex de body/header é o recorte US-25;
  assertions sobre extração regex de body seguem a US-36 acima.

Os **30 pares `header_matches_*`** com Hurl **8.0.0** estão classificados na
[matriz diferencial](compatibility.md#regex-de-headers--us-24--issue-46).
Primeira ocorrência em vez de lista, esperado estático em vez de interpolado,
validação antecipada e códigos são contratos próprios, não equivalência geral
de engine/gramática. Corpus sintético não é homologação da suíte real.

<a id="esperados-dinâmicos"></a>

## Esperados dinâmicos — US-30 / issue #60

`--render-asserts` é um opt-in por invocação e uma evolução explícita do
contrato HTTPGo. Sem a flag, permanece o perfil **literal** descrito abaixo.
Com a flag, operandos JSON de igualdade, comparação, contagem e pertencimento
podem usar variáveis iniciais, capturas confirmadas de entradas anteriores e
capturas provisórias válidas da própria resposta (US-34):

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

Execute com `httpgo test order.httpgo --render-asserts --var base_url=http://localhost:8080`
contra POST/GET retornando `{"number":9007199254740993,"name":"Pizza"}`.
[Exemplo versionado](../testdata/scenarios/scenario_render_asserts.httpgo).
Valores completos
preservam tipo e precisão (incluindo `9007199254740993`); dentro de strings,
escalares viram texto escapado, nunca código ou um segundo template. Arrays e
objetos só cabem em valor JSON completo e nos operadores que já os aceitam.
`count` exige inteiro não negativo e comparação ordenada exige número.

Todo erro estático continua impedindo HTTP do arquivo inteiro. O esperado da
assertion alcançada é resolvido e validado **antes** de consultar a resposta;
variável ausente ou tipo esperado incompatível é avaliação **2**, enquanto
comparação divergente é **1**. Status e assertions anteriores têm precedência;
capturas válidas da própria resposta estão disponíveis provisoriamente (US-34). Valores secretos e
seus derivados ficam protegidos antes dos callbacks, sem reter planos nos
resultados. A flag não altera estrutura de consultas nem ordem de publicação;
a US-31 acima também habilita operandos de igualdade em consultas.

O mesmo opt-in resolve os esperados textuais de headers/body e os padrões
`matches` JSON/header. Operandos continuam exigindo strings entre aspas, com
os escapes já aceitos em cada contexto; números, booleanos e `null` inseridos
viram texto escalar, nunca arrays/objetos serializados. Por exemplo:

```hurl
GET {{base_url}}/orders/{{order_number}}
HTTP 200
[Asserts]
header "X-Order" contains "{{order_number}}"
body not contains "{{forbidden}}"
jsonpath "$.name" matches "^{{order_pattern}}$"
```

Metacaracteres inseridos participam da regex **sem escape automático**. Um
padrão sem referências é compilado antes da rede; um padrão dinâmico é compilado
somente ao alcançar a assertion, antes da consulta. Padrão final inválido ou
não suportado é erro de avaliação **2**, sem expor o erro bruto da engine.
A substituição não é recursiva, inclusive em regex. Ausência/vazio, pertencimento
JSON tipado e primeira ocorrência de header mantêm as regras existentes.
Negação não aprova esperado inválido; headers sensíveis ocultam ambos os lados.
A flag não habilita `body matches`, operadores novos, consultas inteiras/estrutura
dinâmica ou interpolação de padrões de extração/captura. A query regex da US-36
renderiza somente esperados. Operandos de filtros seguem US-31.

<a id="igualdade-json"></a>

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

Salve como `product.httpgo` e execute
`httpgo test product.httpgo --var base_url=http://localhost:8080` contra uma API que
retorne `{"name":"Pizza","price":4500,"active":true}`. Exemplo versionado:
`testdata/scenarios/scenario_assert_json.httpgo`; fluxo autenticado com headers e ID
`9007199254740993` em `testdata/scenarios/scenario_assert_json_auth_products.httpgo`, ambos nos E2E.

- Consulta estática usa exatamente o subconjunto de capturas: `$`, propriedades
  por ponto, índices não negativos e `[*]` de arrays combináveis (US-27 acima).
  Inclui filtros estáticos por igualdade US-28; sem interpolação de consulta,
  wildcard de objeto, outros filtros ou descida recursiva.
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
- Ordem US-34: resposta consumida/fechada → status numérico, se declarado →
  capturas provisórias → assertions de headers e JSON na ordem escrita →
  confirmação → callback. Nas assertions, JSON é decodificado sob demanda e
  descartado depois; capturas podem exigir JSON antes delas. A primeira falha
  interrompe o fluxo, sem confirmação nem próximos envios. `HTTP *` dispensa só comparação
  numérica; contagens e prioridade de erro de escrita (2) permanecem iguais.
- Diagnóstico localiza a assertion (arquivo/linha/coluna), identifica consulta e
  apresenta esperado/real como JSON sanitizado: string com aspas, número, booleano,
  null ou estrutura. `ausente` difere de `null` e `""`. Nunca despeja o body,
  documento inteiro não selecionado ou coleção de headers. O resultado/callback
  não guarda body, documento decodificado ou literal esperado bruto.
  Assertions de body seguem a mesma ordem e proteção (seção US-21), com texto
  decodificado separadamente e sob demanda; erro de charset é avaliação de body (2).
- Segredos marcados, capturas anteriores secretas e derivados/escapes conhecidos
  são removidos de consulta, posição e operandos antes da coleção/formatação.
  Se houver **captura secreta pendente na mesma resposta**, esperado e real de
  falhas de headers **e** JSON ficam integralmente ocultos, inclusive nos metadados
  do callback. A inspeção de marcação não antecipa seleção nem confirmação;
  a extração normal do lote, porém, precede todas as assertions (US-34).
  A ausência continua identificável junto à ocultação. Essa política não reconhece segredos
  arbitrários não marcados e não substitui os limites de segurança documentados.

### Padrões textuais — US-15 / issue #36

**Ampliação intencional:** `[Asserts]` aceita `jsonpath "consulta" matches "padrão"`, somente para o valor selecionado por JSONPath. Não amplia a consulta JSONPath; a US-24 reutiliza a regex para headers (seção acima), mas `body matches` e variáveis continuam fora do recorte; extração pela query `regex` segue US-36.

```hurl
GET {{base_url}}/products/1
HTTP 200
[Asserts]
jsonpath "$.name" matches "^Pizza"
```

O padrão é um literal string JSON em aspas duplas; escapes são interpretados segundo JSON uma vez (`\\\\` produz `\\`, `\\"` produz `"`, e `/` não exige escape). Não são aceitos delimitadores `/.../`, operandos adicionais nem escapes exclusivos de Go na camada JSON. `{{nome}}` dentro do padrão permanece literal, sem renderer. Após decodificar o literal, o padrão segue a sintaxe de `regexp` da biblioteca padrão Go: busca por trecho, classes/grupos/alternativas/repetições/âncoras e opções internas como `(?i)`, `(?m)` e `(?s)`. Lookaround e backreferences não são suportados e são rejeitados explicitamente. Comparação distingue caixa por padrão; `.` não cruza newline sem `(?s)`; Unicode não é normalizado. Padrão vazio corresponde a qualquer string.

String divergente, consulta ausente ou valor selecionado não string falha como assertion (**1**), sem coerção. Ausência e tipo incompatível são diagnosticados separadamente; arrays/objetos não são impressos. JSON inválido e percurso incompatível permanecem erro de avaliação (**2**). Regex inválida/não suportada ou escape JSON inválido rejeita o arquivo inteiro antes de qualquer HTTP, inclusive se a declaração for tardia. Ordem US-34: status → capturas provisórias → assertions na ordem escrita → confirmação; a primeira falha impede confirmação e próximos envios. Padrão e valor são sanitizados, e captura secreta pendente oculta os dois integralmente sem confirmação antecipada. Resultados não conservam padrão operacional nem regexp compilada.

O diferencial executado com Hurl 8.0.0 comprovou `^Pizza` e `Pizza` sem âncoras com código de sucesso em `matches_prefix` e `matches_substring`; `^Pizza$` não corresponde a `Mini Pizza` em ambos, mas usa códigos distintos (HTTPGo 1, Hurl 4). Esta evidência não implica equivalência geral de gramática, escapes ou diagnósticos. Outros padrões e recursos não testados não são anunciados como equivalentes. A US-24 acrescenta `matches` em headers; `matches` em body/variáveis continua fora; interpolação de esperados exige
`--render-asserts` (US-30), sem modificar o perfil literal descrito aqui. Extração por regex de body/header tem o recorte separado da US-25.

<a id="presença-quantidade-e-limites"></a>

### Presença, quantidade e limites — US-14 / issue #34

Exemplo executável: `testdata/scenarios/scenario_assert_structure.httpgo`:

```hurl
GET {{base_url}}/products

HTTP 200
[Asserts]
jsonpath "$.items" count == 3
jsonpath "$.total" >= 1
jsonpath "$.items[0].id" exists
```

Resposta demonstrativa: `{"items":[{"id":null},{"id":2},{"id":3}],"total":3}`.

`jsonpath "$.id" exists` verifica presença: `null`, vazios, false e zero
presentes existem. Propriedade ausente ou índice fora do array falha com 1.
`jsonpath "$.items" count == 3` conta somente elementos do array selecionado,
inclusive elementos null; array vazio tem zero elementos. Aceita comparadores
`==`, `>`, `>=`, `<` e `<=`. O esperado é um número JSON inteiro não negativo,
sem limite de inteiro de máquina; representações exatas como `1.0` e `1e3`
são aceitas. Quantidade negativa/fracionária ou operando em `exists` é sintaxe
inválida (2) antes de qualquer envio do arquivo. Ausência ou tipo não array
falha com 1, sem contar strings/objetos implicitamente. A divergência exibe a
quantidade, nunca o array completo. Percurso incompatível/JSON inválido mantém
erro de avaliação 2. Ordem, proteção de segredos e capturas seguem as regras acima;
captura secreta pendente oculta inclusive as contagens.

`jsonpath "$.total" >= 1` compara números com `>`, `>=`, `<` ou `<=`,
sem coerção de strings. Negativos, decimais, alta precisão e expoentes usam a
mesma representação exata da igualdade, sem arredondamento ou expansão de potências.
Ausência, tipo selecionado não numérico e condição falsa falham com 1. Esperado
não numérico é sintaxe inválida (2) antes da rede; JSON inválido e percurso
incompatível continuam erros de avaliação (2).

Continuam fora do recorte: count em strings/objetos, filtros além da igualdade estática US-28 e interpolação estrutural de consultas. Count em captura JSONPath é suportado na US-33 acima.
Esperados dinâmicos exigem `--render-asserts` (US-30). `matches` aceita strings selecionadas de JSON e a primeira ocorrência de header, nos recortes documentados acima. A gramática de literais
compostos/expoentes difere dos pares observados com Hurl 8.0.0; veja
`compatibility.md`. Os novos pares com Hurl 8.0.0 confirmam que null, vazios,
false e zero presentes satisfazem `exists`. Count negativo/fracionário é rejeitado
na pré-validação aqui, mas chega à assertion no Hurl observado. Quantidade maior
que inteiro de máquina passa no par de ambos; não se deduz precisão numérica geral
da referência. A semântica aprovada não é alterada para forçar equivalência.

<a id="tipos-e-valores-vazios-json"></a>

## Tipos e valores vazios JSON — US-29 / issue #58

```hurl
GET {{base_url}}/session-info
HTTP 200
[Asserts]
jsonpath "$.token" isString
jsonpath "$.attempts" isInteger
jsonpath "$.active" isBoolean
jsonpath "$.issues" isEmpty
```

Os quatro predicados são unários, sem esperado. `isString` aceita inclusive a
string vazia; `isBoolean` aceita true e false. `isInteger` exige número JSON em
**notação inteira**, inclusive negativo e -0, sem limite de inteiro de máquina.
Strings numéricas, decimais (`3.0`) e expoentes (`3e0`) falham, sem coerção.
Igualdade numérica continua exata e considera `3`, `3.0` e `3e0` equivalentes;
`count == 3.0`/`3e0` continua aceitando uma quantidade integral.

`isEmpty` aceita somente string vazia, array sem elementos e objeto sem
propriedades. Whitespace é conteúdo; `[null]`, `{"x":null}`, null, números e
booleanos não passam. Ausência é distinta de null/vazio e falha com 1 em todos
os quatro predicados. JSON inválido ou percurso incompatível mantém erro 2.
Operando extra é sintaxe inválida; `count isInteger` (ou outro novo unário),
negação genérica e outros tipos são não suportados. Todos são rejeitados antes
de qualquer envio do arquivo, inclusive em declaração tardia.

O predicado recebe uma única vez o valor reduzido da consulta: zero resultados
é ausência, um preserva seu tipo, vários formam array sem aplicação por item.
A primeira falha descarta o lote provisório e impede a próxima requisição.
Diagnósticos informam
consulta, predicado, presença/tipo e causa sem imprimir valores ou estruturas;
metadados protegidos e captura secreta pendente seguem a ocultação existente.
`isString` é um atalho para a verificação de tipo via `matches ""`; vazio de
string/array também pode ser validado por `== ""`/`count == 0`. Não há schema,
coerção, outros tipos ou negação genérica.

<a id="polling-local-por-entrada"></a>

## Polling local por entrada — US-26 / issue #50

```hurl
GET {{base_url}}/jobs/123
[Options]
retry: 3
retry-interval: 500
HTTP 200
[Asserts]
jsonpath "$.status" == "READY"
```

`retry` é a quantidade de novas tentativas após a inicial (default `0`);
`retry-interval` é inteiro em milissegundos (default `1000`, zero sem espera
deliberada). Opções estáticas não negativas, sem duplicação, ilimitado ou overflow;
qualquer outra opção é rejeitada antes de todo HTTP do arquivo. A seção pertence
à requisição e não pode seguir a resposta nem ser duplicada.

Somente divergências de status/assertions (`1`) permitem repetir. Transporte,
avaliação/decodificação, captura, preparação, cancelamento ou escrita (`2`)
encerram imediatamente. Cada tentativa repete a requisição completa, inclusive
POST: efeitos podem se repetir, sem garantia de idempotência. Variáveis/caminhos
permanecem; arquivos externos e multipart são relidos/reabertos, sem snapshot dos
bytes se outro processo alterar o arquivo. Cada tentativa começa no método/URL
originais, não no destino de um redirect.

Respostas/fontes são fechadas antes de avaliar/esperar. Ordem US-34: status →
capturas provisórias → assertions → confirmação atômica, somente após aprovação. Proteções de
segredos descobertos não são descartadas. Espera é cancelável, apenas entre
tentativas elegíveis. Cada envio tem seu timeout HTTP; não há timeout global novo,
backoff, retry global ou reexecução do arquivo.

Há uma entrada lógica e um resultado/callback terminal. `[tentativas=N]` indica
os envios iniciados; resumos separam `tentativas_iniciadas`, `respostas_completas`
e `status_comparados` (comparações numéricas, aprovadas ou divergentes). Contadores
legados permanecem lógicos, cada entrada contada no máximo uma vez, pelos fatos
da última tentativa enviada; uma preparação falha não inventa envio. Divergências
transitórias recuperadas não contam como falha final. Esgotamento mantém o último
diagnóstico seguro e retorna `1`; cancelamento na espera retorna `2`.

## Execução HTTP

- Cada requisição é iniciada em ordem. Não há paralelismo ou retry
  implícito do transporte; polling explícito segue a seção anterior. HTTP/1.1 sem conexão persistente evita o replay de
  requisições em conexão reutilizada. HTTP/2 não é habilitado nesta história.
- Timeout total **configurável em YAML, default de 10 segundos** por requisição,
  contado uma vez e compartilhado por conexão/TLS, redirects e consumo de todos
  os bodies. Ajuste por `defaults.timeout`, não por uma flag de timeout.
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
- Bodies intermediários e respostas sem capturas/assertions JSON ou body são consumidos
  em streaming, sem retenção. Somente o body final de entrada com capturas ou
  assertions JSON/body é lido em memória, sob o mesmo prazo, e fechado antes de avaliar
  o status final. Headers isolados continuam sem retenção. Bytes e documento
  decodificado não ficam acumulados nos resultados; memória adicional acompanha
  o body final e suas estruturas.
  Resposta incompleta, erro de leitura/fechamento ou timeout no body não aprova
  a expectativa só porque os headers já chegaram. Não há body em relatório.
- Sem expectativa, inclusive 4xx/5xx é sucesso **sem validação de status**.
  Com expectativa numérica, compara-se igualdade com o status final recebido.
  `HTTP *` aceita esse status sem comparação numérica, mas não ignora assertions/capturas.
- Divergência ou falha de captura/renderização/transporte interrompe as próximas
  entradas do arquivo; outros arquivos seguem conforme `--fail-fast`. Cancelamento
  global interrompe toda a suíte.
  Ctrl+C cancela a requisição em andamento. Evidência de sinal real: Linux;
  macOS/Windows ainda não têm aceite nativo (ver `validation.md`).
- Não há cookie jar, proxy configurável ou leitura automática de HTTPGO_*. O transporte não lê HTTP_PROXY/HTTPS_PROXY nesta história.

## CLI e saída

`httpgo --help` e `httpgo test --help` exibem a mesma ajuda rápida em português:
resumo da ferramenta, uso (`httpgo test <arquivo-ou-diretório>... [opções]`),
opções com explicações curtas, indicação da documentação e exemplos ao final.
A lista inclui `--help`, `--version`, `--config arquivo`, `--env nome`,
`--fail-fast`, `--output junit|html`, `--output-file caminho`, `--render-files`,
`--render-asserts`, `--var nome=valor` e `--secret nome`. Relatórios exigem destino;
`--var` recebe apenas valores não sigilosos e `--secret` apenas marca nomes
protegidos, sem fornecer valores secretos.

A ajuda não contém tutoriais, cenários completos nem catálogos da linguagem.
O aprofundamento permanece no [README](../README.md) e neste documento; a ajuda
indica também o [repositório público](https://github.com/francisco-filho/httpgo),
para quem instalou somente o binário. Seus exemplos finais são:

```sh
httpgo test health.httpgo
httpgo test tests/ --env local
httpgo test tests/ --output junit --output-file target/results.xml
```

Solicitar ajuda não carrega configuração, não exige cenários válidos nem faz
requisições HTTP: retorna **0**, texto em stdout e stderr vazio. O texto estático
permanece sem cores/animações em TTY, saída redirecionada e com `NO_COLOR`.
Falha ou escrita curta do resultado mantém **2** e diagnóstico seguro em stderr
quando disponível. Não há nova flag ou ajuda detalhada alternativa; execução,
relatórios e códigos de saída conservam os contratos descritos abaixo.

```text
httpgo test <caminho>... [--config arquivo] [--env nome] [--fail-fast] [--render-files] [--var nome=valor] [--secret nome]
httpgo --help
httpgo test --help
httpgo --version
```

<a id="suítes"></a>

### Suítes — US-16 / issue #38

`test` aceita um ou mais caminhos de arquivos regulares legíveis e diretórios,
inclusive misturados. Descoberta recursiva seleciona cenários nas extensões
publicadas na [seção de compatibilidade](compatibility.md#extensões-de-arquivo--us-38--issue-79),
sem preferência por extensão. Arquivo explícito é validado pelo conteúdo,
independentemente da extensão. Cópias independentes de conteúdo idêntico executam
separadamente, sem deduplicação por nome-base ou conteúdo.

- Normaliza caminhos absolutos/relativos, `.` e `..`, deduplica por identidade do
  filesystem (incluindo hard links) e respeita sua equivalência de caixa, sem
  transformar tudo em minúsculas. Ordena lexicograficamente pelos caminhos
  relativos ao cwd, com `/`, independentemente da ordem dos argumentos. Externos
  podem conter `../`; quando `filepath.Rel` não é possível (unidades distintas
  no Windows), usa absoluto normalizado com `/` para exibição e ordenação.
- Links descobertos são ignorados, inclusive quebrados/cíclicos. Caminho explícito
  que atravessa link no destino ou em ancestral é rejeitado, mesmo em
  `link/../arquivo`. Caminho inexistente/inválido/inacessível ou não regular,
  falha de acesso durante descoberta ou cada diretório explícito sem elegíveis é
  erro **2**, impedindo toda a rede, mesmo com outras entradas válidas. Acesso é
  verificado efetivamente, não inferido apenas de bits de permissão. Não há
  sandbox contra alterações concorrentes do filesystem.
- Configuração global é carregada, resolvida e validada uma vez, antes da rede.
  A descoberta completa termina antes do primeiro envio, mas **não** faz parsing
  antecipado de todos os arquivos. Na vez de cada arquivo, abre, pré-valida o
  conteúdo inteiro e fecha antes de chamar runner. Falha de carga/fechamento ou
  estrutura inválida descarta o cenário inteiro e não executa prefixos.
- Arquivos e requisições são sequenciais. Cada arquivo recebe variáveis e opções
  iniciais imutáveis e sessão nova; capturas não passam ao próximo. Não há cookie
  jar, nem cookies automáticos dentro/entre arquivos. Isolamento não desfaz efeitos
  na API. Payloads permanecem relativos ao respectivo cenário, não à configuração.
- Primeira falha definitiva ou erro interrompe as próximas requisições do arquivo;
  divergências transitórias só são repetidas pelo polling explicitamente configurado.
  Sem `--fail-fast`, continua arquivos seguintes, inclusive após erro de parsing.
  Com a flag sem valor, não inicia os posteriores e os identifica como **não
  executados**, sem analisá-los só para contar requisições. Cancelamento do contexto
  global e falha de apresentação sempre interrompem a suíte. Timeout HTTP continua
  por requisição: erro local permite próximo arquivo sem fail-fast.
- Proteção de segredos conhecidos/derivados e variantes persiste por toda a
  invocação, inclusive em nomes de arquivos, assertions e erros tardios, sem
  compartilhar capturas ou reter sessões anteriores. Invocação seguinte é nova.

Exemplo com três arquivos (fixtures em `testdata/examples/basic/`):

```sh
httpgo test tests/ --env local
httpgo test tests/ --env local --fail-fast
```

Se health/profile passam e products diverge, retorna 1: dois sucessos e uma falha
sem a flag, um sucesso, uma falha e um não executado com ela. Misturar erro e
assertion divergente retorna 2. Sem paralelismo, dependências entre arquivos,
compartilhamento implícito de login, cenário/segredos por stdin ou outras flags.

### Resultados e contagens

`--output junit --output-file <caminho>` seleciona o relatório US-17;
`--output html --output-file <caminho>` seleciona HTML US-35, com nomeação própria
por timestamp, ambos descritos acima. `check` e configurações futuras não estão implementados ou anunciados.
### Progresso interativo — US-41 / issue #88

O modo texto padrão é **compacto somente quando stdout e stderr originais são
TTY**. Writers sem terminal reconhecido ou redirecionamento de qualquer um dos
dois preservam integralmente o modo detalhado anterior, inclusive streams e
contagens. A seleção ocorre antes dos wrappers de proteção. Ajuda, versão,
JUnit e HTML não usam progresso nem resumo compacto.

Cada arquivo alcançado aparece em stderr antes de sua carga, com nome-base e
extensão, barra e `0/?`. Após parsing completo e fechamento, mostra `0/N` antes
de qualquer envio. Cada requisição, com suas capturas/validações, é um step;
o contador avança uma vez por desfecho definitivo, inclusive erro pré-envio.
Retentativas pendentes não avançam o contador nem aumentam o total. A barra
representa avanço, não aprovação, e só muda por eventos, sem animação temporal.

Só a linha corrente é atualizada. Linhas anteriores permanecem visíveis e não
são repetidas ao final; nomes-base iguais não agrupam arquivos distintos.
Finalização substitui a barra por `OK` ou `ERRO`. Qualquer divergência/erro recebe
`ERRO`, e o diagnóstico seguro é emitido imediatamente abaixo, antes do próximo
arquivo, com ordinal do step e posição quando disponíveis. Falha de leitura ou
parsing não inventa step nem total: mostra `ERRO 0/?`. Arquivos não alcançados
mostram `NÃO EXECUTADO 0/?`, sem carga só para contar suas entradas.

```text
- health.httpgo  OK  1/1
- products.httpgo  ERRO  4/10
  step 4, "products.httpgo":12:1: status divergente: esperado 200, recebido 503; execução interrompida
```

Progresso e diagnósticos vão a **stderr**, somente o resumo final a **stdout**,
precedido por uma linha separadora de 47 hífens:

```text
-----------------------------------------------
Testes executados: 3
Tempo: 6s
Tempo médio por teste: 2s
Sucesso/erros: 2/1
```

- `Testes executados`: arquivos com carga tentada (`Processed`), inclusive falha
  de leitura/parsing ou preparação sem envio; não equivale a `Executed`.
- `Sucesso/erros`: aprovados (`Passed`) e não aprovados (`Failed + Errors`).
- `Tempo`: duração já medida da suíte; média = duração / tentados, ou
  `não aplicável` com zero tentados.
- `Não executados: N`: somente quando `Skipped > 0`; excluídos dos tentados e da
  média. Uso/configuração/descoberta inválidos continuam sem resumo.

Cores são habilitadas na ausência de `NO_COLOR`; sua presença, mesmo vazia,
desativa cores, mas mantém rótulos, barra textual e atualização da linha.
Proteção incremental cobre cada atualização, nome e diagnóstico; não acrescenta
bodies/credenciais nem promete apagar texto anterior à descoberta do segredo.
Falha/short write em carga, total, avanço, finalização, diagnóstico ou resumo é
apresentação global: código 2, sem próximos envios ou novas tentativas no writer
falho. Interrupção, continuação, fail-fast e cancelamento permanecem inalterados.

A apresentação interativa é uma **mudança deliberada de contrato**; os contadores
estruturados, JUnit/HTML, texto redirecionado e códigos 0/1/2 não mudam.

### Texto detalhado preservado por redirecionamento

As linhas e contagens a seguir descrevem o **modo texto legado**, selecionado
quando pelo menos um dos streams não é TTY.

Cada entrada iniciada (ou que falha na renderização pré-envio) tem uma linha de
resultado com o método declarado, URL segura e uma
classificação: `status validado`, `sem validação de status`, `status aceito por HTTP *`,
`status divergente`, `assertion de header divergente`, `assertion JSON divergente`,
`erro de avaliação JSON` ou `erro de transporte`; uma entrada que falha antes do envio recebe
`erro de renderização`. Em sucesso, inclusive sem expectativa, a linha começa
diretamente com o método, sem prefixo de arquivo/linha/coluna. Apenas divergências
e erros de transporte/renderização/captura/headers/JSON/body incluem esse prefixo.
A falha de header/JSON/body aponta à declaração da assertion. Divergências mostram esperado e
recebido. Falha de transporte não inventa status. `erro de captura` mostra o status
real concluído e se ele foi validado; o diagnóstico em stderr localiza a declaração/consulta.

```text
GET http://localhost:8080/health: status validado (esperado 200, recebido 200)
GET http://localhost:8080/error: sem validação de status (recebido 500)
"health.httpgo":1:1: GET http://localhost:8080/health: status divergente (esperado 200, recebido 503)
```

O resumo identifica o arquivo normalizado, sem linha/coluna. Desde a US-16,
nomes em resumos e diagnósticos são relativos ao cwd com `/` (ou fallback absoluto
entre unidades), mesmo quando o argumento original foi absoluto. Referências de
payload conservam sua localização própria.

A linha de cada resultado é escrita **assim que a requisição termina, antes de
iniciar a próxima**, sem esperar o arquivo inteiro terminar. O parsing completo
do arquivo continua ocorrendo antes de suas requisições; uma falha de
pré-validação gera somente resumo/diagnóstico daquele arquivo. Uma resposta só está
concluída depois de consumir e fechar o body; receber headers não basta para
exibir validação aprovada. As requisições continuam sequenciais, sem goroutines
adicionais para apresentar os resultados. Os códigos de saída são preservados.
A apresentação incremental não reimprime resultados no final.

O resumo de cada arquivo é escrito uma única vez antes de iniciar o seguinte,
inclusive quando houve divergência, captura, renderização, transporte ou cancelamento.
Uma linha `resumo suíte:` agrega a invocação, também para arquivo único. Não há
novo formato de saída: a evolução adiciona campos ao resumo por arquivo e uma
linha agregada. Os contadores por arquivo em stdout continuam:

- `total`: entradas validadas pelo parser;
- `iniciadas`: entradas com pelo menos uma chamada ao executor — **não comprova
  recebimento pelo servidor**;
- `concluídas`: entradas cuja última tentativa enviada teve resposta totalmente
  consumida/fechada, inclusive divergentes;
- `validadas`: entradas com comparação de status aprovada na última tentativa enviada;
- `sem_validação`: respostas concluídas sem comparação numérica (sem expectativa ou `HTTP *`);
  a linha individual distingue esses dois casos. Não significa que headers/JSON não foram avaliados;
- `não_iniciadas`: entradas não iniciadas por interrupção.
- `tentativas_iniciadas`: total de chamadas reais ao executor;
- `respostas_completas`: total de respostas completamente consumidas/fechadas;
- `status_comparados`: comparações numéricas realizadas, aprovadas ou divergentes.

As três métricas novas também são somadas no resumo da suíte. Redirects não são
novas tentativas; preparação falha não soma envio. Os contadores anteriores
continuam contando uma entrada no máximo uma vez, pelos fatos da última tentativa
enviada; cancelamento no intervalo é erro final sem apagar esses fatos HTTP.
O resumo por arquivo acrescenta `resultado` (`sucesso`, `falha de assertion`,
`erro` ou `não executado`) e `duração` não negativa de carga/execução. `validadas`
**não** comprova sucesso completo da requisição: uma assertion de header/JSON ou
captura posterior pode falhar após o status passar.

Erro de pré-validação tem zero `iniciadas`/`concluídas` e resultado `erro`.
`total` e `não_iniciadas` são `desconhecido` quando não houve parsing completo,
inclusive para arquivos não executados; não se inventa zero como quantidade de
requisições planejadas. Uso/configuração/descoberta inválidos impedem a execução
inteira e não apresentam resumos.

O agregado informa:

- `arquivos`: selecionados após deduplicação; `processados`: carga tentada;
  `executados`: pelo menos uma requisição iniciada;
- `sucessos`, `falhas_assertion`, `erros`, `não_executados`: estados dos arquivos,
  não derivados do contador de status aprovados;
- `requisições_conhecidas`: soma de `total` apenas dos arquivos com parsing completo;
  `arquivos_contagem_desconhecida`: arquivos com erro de carga/parsing ou não executados;
- `iniciadas`, `concluídas`, `validadas`, `sem_validação`, `não_iniciadas`: soma dos
  contadores conhecidos do runner. `não_iniciadas` não inclui entradas desconhecidas;
- `requisições_sucesso`: requisições concluídas sem qualquer falha, inclusive sem
  assertion; `requisições_falha_assertion`: divergências; `requisições_erro`: entradas
  com erro reportado, inclusive pré-envio. Erro de parsing não inventa uma entrada;
- `duração`: carga/execução/apresentação sequencial da suíte, sem configuração ou
  descoberta. Duração e contagens não são um timeout global.

Diagnósticos ficam em stderr e incluem posição quando identificável. Erros
brutos da rede, Location, linha de entrada e bodies nunca são impressos.

| Exit code | Significado |
|---|---|
| 0 | todos os arquivos concluídos sem falha/erro; inclui requisições sem expectativa |
| 1 | pelo menos uma divergência de status ou assertion de header/JSON/body, sem erros de categoria 2 |
| 2 | uso, configuração, arquivo, parsing, avaliação JSON/body, captura, renderização, transporte, TLS, timeout, cancelamento ou escrita |

Na suíte, **2 prevalece sobre 1**. Erro ao escrever stdout/stderr também é
categoria 2; não transforma divergência em sucesso. Falha/short write em resultado,
resumo por arquivo, diagnóstico ou agregado encerra toda a suíte antes de novos
envios, preserva o que já foi escrito e não tenta novamente o writer que falhou. O erro é diagnosticado de forma segura em stderr, sem ecoar a causa bruta.
O modo legado usa texto simples sem cores/animações, independentemente de
NO_COLOR. O compacto interativo segue a seleção e os streams descritos acima.
O E2E comprova pipe, redirecionamentos parciais e dois PTYs Linux reais.

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
- Redirects são automáticos mas limitados à mesma origem; o diferencial legado usa
  `--location --max-redirs 10 --max-time 10 --retry 0` no Hurl. Pares `polling_*`
  deixam a política na entrada, exceto `polling_cli_zero`, que comprova que a
  opção local Hurl prevalece sobre `--retry 0`. Erro de decode/captura não repete
  aqui, mas o Hurl 8.0.0 repetiu nos pares observados; veja `compatibility.md`.
  A comparação aproxima as políticas compartilhadas. Essa aproximação não torna os motores equivalentes.
- Variáveis CLI tipadas são extensão HTTPGo, sem recursão. Não presuma as mesmas
  regras na CLI Hurl: o corpus diferencia strings explicitamente delimitadas,
  estruturas e conversão textual. Interpolação escalar contextual em JSON possui
  pares compartilhados observados, sem anunciar equivalência geral.
- Nomes pontuados são aceitos aqui, mas `api.baseUrl` falhou no Hurl 8.0.0.
  Duplicações/conflitos são erros aqui; Hurl permite sobrescrita no corpus.
  Hurl aceita templates em nomes de headers/chaves JSON; esta entrega não.
- Capturas aceitam todos os tipos JSON desde a US-06 e renovação exata desde a US-34.
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
  das assertions; aqui o esperado é literal por padrão (opt-in US-30 para renderizar)
  e headers/JSON usam o lote provisório antes da confirmação (US-34).
- Igualdade JSON aqui aceita literais esperados compostos e expoentes exatos;
  Hurl 8.0.0 rejeitou array/objeto e `125e-2` nos pares observados. Tipos escalares,
  precisão inteira e decimais têm pares compartilhados, sem equivalência geral.
  Avaliação inválida/percurso incompatível retorna 2 aqui contra 4 no Hurl;
  ausência de propriedade/índice retorna 1 aqui contra 4. Veja `assert_json_*`.
- `--render-files` é uma **extensão HTTPGo** implementada na US-10, não parte do
  Hurl. Seus testes ficam separados do corpus diferencial em modo bytes, que não
  recebe essa flag. Configuração YAML hierárquica é a extensão US-11, também
  separada do diferencial e sem promessa de equivalência com configuração Hurl.

Veja a matriz e evidências limitadas ao corpus em `compatibility.md`.
