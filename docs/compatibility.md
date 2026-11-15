# Matriz de compatibilidade — US-01 a US-05

## Escopo da evidência

Referência fixada: **Hurl 8.0.0**. O diferencial executa o binário, não importa nem
lê seu código-fonte. `testdata/compatibility/` é contrato versionado: cada `.hurl`
tem um `.json` com classificação, exit code e paths observados em cada ferramenta.
Os pares novos da US-02 também registram método, bytes do body, `Content-Type` e
`X-Probe` recebidos pelo servidor para cada entrada e cada redirect.
Alterar esses arquivos em entregas futuras é mudança de contrato a sinalizar no PR.

```sh
make compatibility
# Atualizar observações intencionalmente (sempre revisar o diff):
go test -tags compatibility ./tests/e2e -run '^TestCompatibility$' -count=1 -args -update
```

Ausência de Hurl ou versão diferente de 8.0.0 causa **falha**, não skip. O alvo
não baixa ou instala Hurl automaticamente. O teste substitui a origem absoluta
`http://example.invalid` por um httptest.Server local em cópia temporária. Isso
é adaptação do harness para os pares estáticos US-01/US-02. Os pares US-03 usam
placeholders no cenário original e o campo `variables` no JSON de referência:
o harness fornece os mesmos valores via `--var` ao httpgo e `--variable` ao Hurl.
Somente a origem do valor da variável é adaptada ao servidor local; o cenário
parametrizado não é editado. Os novos pares incluem `uri` (RequestURI recebido)
para provar que encoding explícito não foi duplicado.

Os comandos Hurl usam `--no-color --no-output --location --max-redirs 10
--max-time 10 --retry 0 --no-cookie-store --no-proxy '*'`. Não se compara texto
bruto de saída nem assume igualdade de códigos: a comparação observa aceitação,
falha, ordem, contatos efetivos e, nos pares US-02, dados efetivamente enviados,
conforme cada par esperado. Os testes próprios
httpgo verificam conteúdo informativo e sanitização em ambos os streams.

## Matriz observada (Linux amd64)

| Corpus | httpgo | Hurl 8.0.0 | Classificação |
|---|---|---|---|
| `sequence` | 0; /health, /missing | 0; mesmos paths | compartilhado: GETs, comentário próprio, linhas vazias, 200/404 |
| `boundary` | 0; /boundary | 0; mesmo path | compartilhado: status 599 |
| `unvalidated` | 0; /error retorna 500 | 0; mesmo path | compartilhado: sem expectativa aceita 500 |
| `mismatch` | 1; somente /unavailable | 4; mesmo path | semântica de divergência/fail-fast compartilhada; exit codes diferentes |
| `redirect` | 0; /redirect, /health | 0; mesmos paths com flags alinhadas | compartilhado: status final validado |
| `inline_comment` | 2; zero envios | 0; /health | restrição intencional: comentário inline rejeitado |
| `http_version` | 2; zero envios | 0; /health | restrição intencional: resposta versionada rejeitada |
| `post` | 0; /health | 0; mesmo path | compartilhado: POST sem body (contrato ampliado na US-02) |
| `cross_origin` | 2; somente /cross | 0; /cross, /health | política intencional: localhost difere de 127.0.0.1, mesmo que resolva igual |

### Ampliação US-02 — mudança de contrato sinalizada

`post.json` deixa de esperar rejeição e passa a esperar sucesso: **mudança
intencional do contrato de compatibilidade**, autorizada pelo escopo da issue #6.
Os demais pares US-01 permanecem com as mesmas observações.

| Corpus novo | Resultado observado nas duas ferramentas | Diferenças |
|---|---|---|
| `json_object` | POST, header declarado e bytes exatos; exit 0 | nenhuma nos campos observados |
| `json_multiline` | PUT com objeto multilinha/escapes/delimitadores em string, seguido de GET 404; exit 0 | Hurl infere Content-Type; httpgo não |
| `json_array` | PATCH com array multilinha e bytes exatos; exit 0 | Hurl infere Content-Type; httpgo não |
| `json_string`, `json_number`, `json_true`, `json_false`, `json_null` | valor escalar preservado e GET seguinte separado; exit 0 | Hurl infere Content-Type; httpgo não |
| `method_sequence` | POST → PUT → PATCH (500 sem expectativa) → DELETE sem body → GET 404; exit 0 | Hurl infere Content-Type nos bodies; httpgo não |
| `post_redirect_301`, `_302`, `_303` | POST inicial, destino GET sem body; headers declarados mantidos; exit 0 | nenhuma nos campos observados |
| `post_redirect_307`, `_308` | POST e bytes preservados no destino; exit 0 | nenhuma nos campos observados |
| `json_invalid_late` | exit 2, zero envios, inclusive GET anterior | rejeição integral compartilhada |
| `get_body` | httpgo exit 2 sem envio; Hurl exit 0 com GET e JSON | restrição intencional: GET sem body |

A ausência de `Content-Type` automático é uma diferença explícita: para obter
esse comportamento compartilhado, declare `Content-Type: application/json`, como
na demonstração. A matriz não generaliza a política de redirects para todos os
métodos do Hurl: o diferencial fixa POST; os testes próprios httpgo também cobrem
PUT/PATCH/DELETE conforme a semântica definida na US-02.

### Ampliação US-03 — contrato de variáveis sinalizado (issue #11)

Dez novos pares `variables_*.hurl/.json` ampliam o contrato para **35 pares**.
Nenhuma observação antiga US-01/US-02 foi alterada. A aceitação de templates em
URL/valores de headers é uma ampliação intencional; o corpus de cenários renomeia
as antigas rejeições globais para positivos, mantendo negativos de body/nomes.
Todos os valores usados são fictícios ou não sigilosos, nunca credenciais reais.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença |
|---|---|---|---|
| `variables_text` | 0; /health | 0; /health | texto vazio, `=`, números/booleanos textuais, composição e header não recursivo idênticos |
| `variables_encoding` | 0; URI `/a%2Fb?q=a/b` | 0; mesma URI | encoding fornecido não é duplicado; separadores reservados preservados |
| `variables_missing` | 2; somente /health | 3; somente /health | falta variável antes do segundo envio; códigos próprios |
| `variables_url_space` | 2; sem envio | 3; sem envio | ambos rejeitam espaço em URL renderizada, sem encoding automático |
| `variables_hierarchical` | 0; /health | 3; sem envio | `api.baseUrl` funciona aqui; Hurl não resolve esse nome pontuado no exemplo |
| `variables_duplicate` | 2; sem envio | 0; /health, X-Probe=second | httpgo rejeita repetição; Hurl sobrescreve |
| `variables_conflict` | 2; sem envio | 0; /health | httpgo rejeita escalar/descendente mesmo sem uso; Hurl aceita atribuições |
| `variables_malformed_late` | 2; sem envio | 3; somente /health | nome com hífen é inválido estruturalmente aqui; Hurl o trata como referência ausente após primeiro envio |
| `variables_body` | **0; POST /health na US-04** | 0; POST /health | bytes idênticos; antiga restrição US-03 removida intencionalmente na issue #13 |
| `variables_header_name` | 2; sem envio | 0; X-Probe=value | nomes de headers permanecem estáticos aqui; Hurl interpola |

### Ampliação US-04 — JSON tipado (issue #13)

**Mudança intencional de contrato:** `variables_body.json` passa de exit 2/zero
envios para exit 0/POST com body `{"probe":"fictitious"}`. O arquivo `.hurl` não
mudou. A antiga rejeição global de body no corpus de cenários foi limitada a chaves
JSON; corpos e valores string agora são aceitos nos contextos previstos. Os demais
34 pares anteriores permanecem com as mesmas observações. Seis pares novos
ampliam a matriz para **41 pares**, com os mesmos argumentos fornecidos a ambos
os executáveis; todos usam `Content-Type` explícito para isolar a tipagem.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença observada |
|---|---|---|---|
| `typed_values` | 0; JSON com string, número, booleano e null | 0; mesmos bytes/header | interpolação contextual escalar compartilhada neste par; `X-Probe=4500-true-null` |
| `typed_forced_string` | 0; `{"code":"001","text":"001"}` | 0; `{"code":001,"text":"001"}` | extensão CLI HTTPGo: string delimitada decodificada preserva tipo; Hurl enviou `001` sem aspas no valor completo, que não é JSON válido |
| `typed_composite` | 0; objeto opaco com array e número de alta precisão | 3; zero envios | tipo composto na CLI é extensão própria; Hurl rejeita este valor de variável |
| `typed_root` | 0; array raiz, depois GET /missing | 3; zero envios | HTTPGo preserva tipo e fronteira do placeholder raiz; CLI Hurl rejeita este composto |
| `typed_text_rejected` | 2; zero envios | 0; `{"text":"[1,true]"}` | HTTPGo não converte array em texto implicitamente; CLI Hurl fornece texto neste caso |
| `typed_key_rejected` | 2; zero envios | 0; `{"static":1}` | restrição HTTPGo: chaves permanecem estáticas |

A tipagem de `--var` é **extensão HTTPGo**, não comportamento imputado à CLI Hurl.
Os testes próprios também verificam escapes, Unicode, composição, não recursão,
objetos opacos, conflitos, segredo após decodificação e JSON final antes do envio.
Não se deduz suporte geral a templates de raiz/estruturas no Hurl a partir de uma
falha da sua CLI em aceitar o valor; registra-se somente o comportamento observado.

Estas observações **não** anunciam compatibilidade geral com Hurl ou equivalência
dos motores net/http e libcurl. Timeout, certificados, pooling, headers implícitos,
DNS e diagnósticos podem diferir. Templates em chaves JSON/nomes de headers,
seções além de `[Captures]`, bodies externos/binários/multipart, headers de enquadramento e
demais construções fora de escopo continuam explicitamente não suportadas.

## Ampliação US-05 — capturas string (issue #14)

Nove pares `captures_*` ampliam o contrato para **50 pares**. Os **41 pares
anteriores não mudaram**. No corpus de cenários, a antiga rejeição global
`unsupported_section_captures.hurl` virou `invalid_capture_empty.hurl`: a seção
agora é suportada, mas vazia continua inválida. Negativos específicos mantêm
wildcard, filtro, descida recursiva, índice negativo e outras queries rejeitados
antes da rede. Esta ampliação intencional deve ser sinalizada na revisão/PR.

O harness retorna JSON fictício em `/auth/login`; `response_json` parametriza a
resposta de `/capture` por par. Além dos campos anteriores, `authorization` registra
o token **efetivamente recebido** pela API quando presente. Não são segredos reais;
a saída protegida da ferramenta é verificada pelos E2E próprios, não por este
registro de tráfego fictício.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença observada |
|---|---|---|---|
| `captures_login` | 0; login e POST protegido | 0; mesmos requests | token no path/Authorization/strings JSON; arrays e string vazia; X-Probe `array-token\|` |
| `captures_literal` | 0; body `{"value":"null","text":"null"}` | 0; body `{"value":null,"text":"null"}` | string capturada na raiz conserva tipo aqui; Hurl reinterpreta no valor completo |
| `captures_root` | 0; seleção `$[0].token` sem expectativa | 2; zero envios | HTTPGo permite seção sem `HTTP`; Hurl exige resposta esperada para a seção |
| `captures_dotted` | 0; token recebido em X-Probe | 3; somente login | nome pontuado resolve exatamente aqui; Hurl não resolve o exemplo |
| `captures_collision` | 2; somente login | 0; login e /health com token novo | HTTPGo não sobrescreve variável inicial; Hurl sobrescreve |
| `captures_missing` | 2; somente login | 3; somente login | caminho ausente interrompe; códigos próprios |
| `captures_non_string` | 2; somente /capture | 0; /capture e /health, X-Probe `123` | restrição HTTPGo: capturas somente string, sem coerção numérica |
| `captures_unsupported` | 2; zero envios | 0; /health, login e /health | wildcard reconhecido e não suportado impede todo envio HTTPGo |
| `captures_mismatch` | 1; somente /unavailable | 4; mesmo path | status divergente prevalece sobre captura de body não JSON |

Os testes próprios httpgo cobrem também strings semelhantes a placeholders,
controles/Unicode/escapes, URL/headers inválidos após captura, múltiplas capturas,
conflitos hierárquicos e de lote, isolamento de execução e proteção dinâmica.
Não se deduz compatibilidade geral de JSONPath/capturas a partir desses pares.

## Extensões e restrições

Interpretação tipada de literais JSON na CLI é uma extensão implementada na US-04,
com mudança intencional do contrato textual temporário da US-03. Não há conversão
implícita de objetos/arrays em texto nem navegação de propriedades.

YAML hierárquico e interpolação em arquivos externos são extensões futuras do
httpgo, **não capacidades do formato Hurl implementadas nesta história**.
Não há configuração YAML ou interpolação **externa** nesta entrega. A gramática menor,
saída/exit codes próprios e restrição de redirects estão detalhados em `subset.md`.

## Ambiente usado

A execução local usou Hurl 8.0.0 com libcurl 8.18.0. O binário da sandbox emite um
aviso de versionamento de libcurl em stderr ao iniciar; o harness não o confunde
com falha de HTTP ou altera os resultados para escondê-lo. A matriz acima passou
contra esse binário. Evidência de portabilidade e gate pendente: `validation.md`.
