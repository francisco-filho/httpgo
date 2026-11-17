# Matriz de compatibilidade — US-01 a US-16, US-20 a US-27b

## Preservação de JSONPath — US-27b / issue #54

Integração interna de `github.com/ohler55/ojg/jp` **v1.28.9** limitada à navegação
de projeções já entregues. Caminhos simples continuam nativos; seleção estática,
sem flag de avaliador ou fallback. Não há ampliação de gramática, mudança de
referência Hurl **8.0.0** nem alteração dos **293 pares** existentes. O gate usa
os mesmos arquivos/observações, sem regeneração para acomodar a biblioteca.

Pares `projection_cardinality`, `projection_*_capture`, `projection_scalar_count`
e `projection_strict_*` continuam o contrato de redução, ausência/null, ordem,
arrays selecionados, captura tipada e percurso estrito. Diferenças deliberadas
com Hurl descritas na seção US-27 abaixo permanecem: ramos incompatíveis são erros
2 aqui, e reuso de coleção capturada como JSON completo não implica equivalência
com Hurl. Nenhuma observação anterior foi normalizada ou reclassificada.

Regressões próprias ampliam precisão além de `2^53` e `int64`, decimais próximos,
formas equivalentes e expoentes extremos em seleções/assertions/capturas. Isso
comprova o contrato HTTPGo, não igualdade numérica ou filtros de Hurl/OjG em geral.
Apenas `query.Decode` usa `UseNumber`; `jp` não decodifica nem compara números.
O [exemplo de precisão](../testdata/scenarios/scenario_projection_precision.hurl)
é executado por binário real e HTTP local, incluindo resposta alterada com falha 1.
Funções, filtros, interpolação estrutural e demais recursos fora do recorte
continuam erros estáticos 2, inclusive em entrada tardia e antes de todo HTTP.
Evidências executadas em [validation.md](validation.md#us-27b--issue-54--história-0027b).
Corpus sintético não homologa suíte real nem anuncia conformidade RFC integral.

## Polling — US-26 / issue #50

**Ampliação intencional do contrato:** `[Options]` local com `retry` e
`retry-interval`. São **10 novos pares `polling_*`**, total **283 pares**,
observados contra **Hurl 8.0.0** em Linux. Os **273 pares anteriores permanecem
inalterados**, sem regeneração. Novos goldens foram observados/revisados somente
com filtro `polling_*`; métodos, ordem, headers e bytes são comparados, não tempo,
stdout ou códigos numéricos idênticos de falha. Nenhuma fonte Hurl foi utilizada.

O harness ganhou `response_sequence`, consumida somente em `/poll`; após o fim,
a última resposta se repete. O índice é reiniciado independentemente antes do
segundo executável, assim como paths/requests. Fixtures estáticas antigas mantêm
seu comportamento. Os novos pares usam intervalo zero, sem igualdade temporal.

| Par `polling_*` | HTTPGo | Hurl 8.0.0 | Classificação observada |
|---|---|---|---|
| `ready` | 0; 3 × /poll → /health | igual | duas divergências JSON, terceira aprovação e continuação |
| `exhausted` | 1; 4 × /poll | 4; mesmos paths | inicial + 3 retries, diagnóstico final, sem próxima entrada |
| `status` | 0; 2 × /poll → /health | igual | 503 esperado como 200 diverge, depois aprova |
| `post` | 0; 2 × POST /poll → GET /health | igual | mesmo JSON, Content-Type explícito e X-Probe reenviados; pode repetir efeitos |
| `zero`, `omitted` | 1; 1 × /poll | 4; mesmo path | zero explícito e ausência de retry não repetem |
| `cli_zero` | 0; 2 × /poll → /health | igual, mesmo com --retry 0 | **opção local retry prevalece sobre flag global Hurl** no par observado |
| `decode` | 2; 1 × /poll | 0; 2 × /poll → /health | **diferença deliberada:** erro JSON não é condição elegível aqui; Hurl repetiu |
| `capture` | 2; 1 × /poll | 0; 2 × /poll → /health | **diferença deliberada:** captura ausente encerra aqui; Hurl repetiu |
| `invalid_late` | 2; zero envios | 2; zero envios | retry fracionário na entrada tardia rejeitado antes da rede |

Nos pares antigos o comando conserva `--retry 0`. Nos novos, a opção global é
omitida, exceto `cli_zero`, que usa `hurl_retry: 0` para registrar a precedência
real: a flag **não** neutralizou a opção local no Hurl 8.0.0. HTTPGo não oferece
retry global. A política HTTPGo é mais estreita: só divergência de status/assertion
(1) pode repetir; transporte, avaliação, captura, preparação, cancelamento e
escrita (2) encerram imediatamente. Não se deduz equivalência da política de
outros erros Hurl não presentes no corpus.

Default de intervalo (1000 ms), cancelamento, limites representáveis, duplicação,
proteção de segredos descartados, releitura/reabertura de arquivos, fechamento,
redirects, prazo HTTP por tentativa e métricas lógicas/reais são comprovados pelos
testes HTTPGo, sem comparar duração com Hurl. Não há retry ilimitado, backoff,
timeout global novo, idempotência de POST ou snapshot de arquivos. Capturas só
publicam após status/assertions aprovados, diferentemente da ordem Hurl já
documentada. Resumos novos são contrato próprio, não cópia da saída Hurl.

Evidência: [validation.md](validation.md#us-26--issue-50--história-0026).
Corpus sintético não representa homologação da suíte real nem migração sem
adaptações. YAML hierárquico e renderização externa continuam extensões HTTPGo.

## Projeção de arrays — US-27 / issue #52

Referência: **Hurl 8.0.0**, somente comportamento observado, sem usar seu código.
Após integrar o polling da US-26, o corpus passa de 283 para **293 pares**:
dez `projection_*` novos e alteração **intencional de contrato** de dois pares
antigos. Os outros **281** pares não
mudam. `-update` foi limitado aos dez novos e aos dois alterados; o gate completo
é executado sem atualização automática. Nomes históricos `*_unsupported` são
preservados para rastrear a mudança, não descrevem o suporte atual de `[*]`.

| Par | HTTPGo | Hurl 8.0.0 | Evidência/classificação |
|---|---|---|---|
| `captures_unsupported` | **0; /health → /auth/login → /health** | 0; mesmos paths | Antes 2/zero envios aqui; wildcard de array em captura agora aceito |
| `assert_json_query_unsupported` | **0; /health → /capture** | 0; mesmos paths | Antes 2/zero envios aqui; wildcard singleton reduz ao número 1 |
| `projection_cardinality` | 0; /capture → /health | 0; mesmos paths | ausência, singleton escalar/null, múltiplos números, array vazio selecionado, múltiplos arrays, ramos ausentes, projeção aninhada e índice após wildcard |
| `projection_scalar_count` | 1; /capture | 4; /capture | singleton número não vira array para count; divergência compartilhada, códigos próprios |
| `projection_missing_capture` | 2; /capture | 3; /capture | zero resultados não permite completar o fluxo deste par; categorias/códigos próprios |
| `projection_nested_capture` | 0; /capture → POST /health com `[3,1,3]` | 3; /capture | HTTPGo preserva ordem/repetições sem placeholders; Hurl interrompe antes do reuso composto |
| `projection_arrays_capture` | 0; POST com `[[],[1],[[2]]]` | 3; /capture | HTTPGo não achata valores selecionados; reuso composto difere do Hurl |
| `projection_empty_array_capture` | 0; POST com `[]` | 3; /capture | array vazio único é capturado e reutilizado aqui; Hurl interrompe antes do reuso |
| `projection_precision_capture` | 0; POST com `[9007199254740993,1.2300e+99]` | 3; /capture | precisão/grafia numérica preservadas aqui; sem presumir equivalência do reuso composto |
| `projection_strict_property` | 2; /capture | 0; /capture → /health | ramo não objeto ao lado de objetos válidos invalida toda seleção aqui; Hurl descarta o ramo neste exemplo |
| `projection_strict_index` | 2; /capture | 0; /capture → /health | índice em não array é erro aqui; Hurl descarta o ramo neste exemplo |
| `projection_strict_wildcard` | 2; /capture | 0; /capture → /health | `[*]` em objeto vazio é erro aqui, não ausência; Hurl aprova not exists neste par |

**Diferenças deliberadas:** a seleção expansiva é estrita sobre tipos incompatíveis;
a negação não pode aprovar um percurso inválido. Ausência de atributo/índice fora
do array, em contraste, elimina apenas o ramo. A redução final distingue número,
null, ausência e array selecionado. Não se anuncia JSONPath completo, wildcard de
objetos, filtros, slices, unions ou descida recursiva. Caminhos simples preservam
seu contrato (incluindo o adapter legado de captura), não foram normalizados para
imitar o Hurl. Capturas compostas continuam a extensão de reuso tipado da US-06.

Diagnósticos expansivos são resumos de presença/tipo/contagem, sem coleção ou
esperado composto; metadados/contagens protegidos ficam ocultos. E2E próprios
comprovam ordem de assertions, atomicidade, segredos pendentes/descobertos,
isolamento, resposta final consumida/fechada, leitura/close incompleto, timeout,
cancelamento e prioridade 2 em erro/short write. Não se exige texto de diagnóstico
idêntico nem códigos iguais aos do Hurl. O corpus é sintético e não homologa a
suíte real de negócio.

## Regex de headers — US-24 / issue #46

**Ampliação intencional do contrato:** `header "nome" matches "padrão"`, com
string JSON literal estática, engine Go da US-15 e primeira ocorrência da US-07.
São **30 novos pares `header_matches_*`**, total **243 pares**, confrontados com
**Hurl 8.0.0** em Linux usando o harness existente e `response_headers`.
Os **213 pares anteriores permanecem inalterados**, sem regeneração.
Fixtures são independentes entre runners; resposta não JSON comprova que regex
de header não exige decode de body. Códigos/paths estão versionados, sem exigir
stdout, diagnóstico ou códigos de falha idênticos.

| Par `header_matches_*` | HTTPGo | Hurl 8.0.0 | Classificação / observação |
|---|---|---|---|
| `session` | 0; /capture → /health | 0; mesmos paths | cookie de sessão com atributos exigidos |
| `session_missing_secure` | 1; /capture | 4; /capture | formato divergente em ambos; códigos próprios |
| `partial` | 0; /capture → /health | 0; mesmos paths | correspondência interna sem âncoras |
| `anchored`, `case` | 1; /capture | 4; /capture | âncoras e distinção de caixa por padrão |
| `option` | 0; /capture → /health | 0; mesmos paths | opção interna `(?i)` no padrão testado |
| `unicode`, `unicode_class`, `groups` | 0; /capture → /health | 0; mesmos paths | Unicode literal/classe de letras, grupos, alternativas e repetições testados |
| `normalization` | 1; /capture | 4; /capture | formas Unicode distintas não são normalizadas |
| `escapes`, `slash` | 0; /capture → /health | 0; mesmos paths | JSON decodificado uma vez: whitespace regex, aspas, barra invertida e `/` literal |
| `empty_pattern`, `empty_value`, `empty_anchor` | 0; /capture → /health | 0; mesmos paths | padrão vazio passa no presente; `^$` passa no vazio |
| `empty_anchor_value`, `missing` | 1; /capture | 4; /capture | âncora vazia falha no não vazio; ausência falha mesmo com padrão vazio |
| `first`, `first_empty` | 0; /capture → /health | 4; /capture | **diferença deliberada:** primeira ocorrência (inclusive vazia) aqui; Hurl recebe lista, não string escalar |
| `second`, `second_after_empty` | 1; /capture | 4; /capture | ambos falham, mas não por equivalência: segunda ocorrência não corrige primeira aqui; Hurl recebe lista |
| `comma` | 0; /capture → /health | 0; mesmos paths | vírgula preservada em uma ocorrência, sem divisão |
| `literal_missing`, `literal_defined` | 0; /capture → /health | 4; /capture | **diferença:** padrão estático aqui, mesmo com variável definida; Hurl tenta interpolar (ausente ou divergente) |
| `malformed` | 2; zero envios | 4; /health → /capture | **diferença:** regex malformada é pré-validada aqui; Hurl reporta regex inválida após enviar |
| `lookaround`, `backreference` | 2; zero envios | 4; /health → /capture | **não suportado:** rejeição estática controlada aqui; Hurl também reportou regex inválida, mas só em avaliação |
| `json_escape`, `extra` | 2; zero envios | 2; zero envios | escape exclusivo de Go na string JSON e operando extra rejeitados antes da rede |
| `unquoted` | 2; zero envios | 0; /health → /capture → /health | **diferença de gramática:** HTTPGo exige string JSON; Hurl aceita delimitadores `/private/` e corresponde |

A primeira rodada dos pares novos mediu diferenças de validação e delimitadores;
as observações foram ajustadas pontualmente ao binário real, sem `-update` global.
Nos pares de regex inválida foram fornecidos headers presentes para não confundir
ausência com validação da expressão no Hurl. Não se anuncia equivalência geral de
engines, escapes, opções, lista/seleção de ocorrências ou interpolação. Primeira
ocorrência, status → assertions na ordem escrita → capturas e códigos 0/1/2 são
contratos próprios, não modificados para imitar Hurl. Não foi usada fonte Hurl.

Padrão/real sensíveis (incluindo Set-Cookie), segredos conhecidos/derivados,
captura secreta pendente, remoção da regex compilada de resultados/callbacks,
isolamento, consumo/fechamento/contexto e falhas de writers são cobertos pelos
testes HTTPGo, não por comparação de diagnósticos com Hurl. Evidências em
[validation.md](validation.md#us-24--issue-46--história-0024).
Assertions regex de body/variável e todas as ocorrências continuam fora
do suporte; extração regex de body/header é a US-25 abaixo. Placeholders permanecem
estáticos, sem `--render-asserts`.
Corpus sintético não é homologação da suíte real. Extensões YAML hierárquico e
interpolação de arquivos externos continuam explicitamente próprias do HTTPGo.

## Extração regex — US-25 / issue #48

**Ampliação intencional de contrato:** `[Captures]` aceita regex de body e um
único sufixo regex em header. São **30 novos pares `capture_regex_*`**, total
**273 pares**, sobre a base com US-24 integrada. Dos 243 anteriores,
**somente `capture_header_filter.json` muda
intencionalmente**: HTTPGo passa de 2/zero envios para 0 e `/capture → /health`,
enviando `X-Probe: literal`. Nenhuma outra observação antiga foi regenerada.
Reutilizam-se `response_headers`, `response_body` (bytes base64) e fixtures
independentes; harness acrescenta somente observação opcional de `Cookie`
recebido, com dados fictícios. Não se compara stdout nem códigos idênticos de falha.

| Par `capture_regex_*` | HTTPGo | Hurl 8.0.0 | Observação real / classificação |
|---|---|---|---|
| `otp` | 0; /capture → /health | 0; mesmos paths | POST JSON envia string `123456` em ambos |
| `cookie` | 0; /capture → /health | 0; mesmos paths | `Cookie: refresh=fiction-token`, sem atributos; body inválido/charset desconhecido não interfere |
| `no_cookie_store` | 0; /capture → /health | 0; mesmos paths | nenhum Cookie implícito; Hurl com --no-cookie-store |
| `first_group` | 0; /capture → /health | 0; mesmos paths | somente `001`, primeiro grupo da primeira correspondência |
| `empty`, `header_empty` | 0; /capture → /health | 0; mesmos paths | grupo participante vazio envia `prepost`; header vazio não é ausente |
| `optional`, `no_group`, `no_match` | 2; /capture | 3; /capture | não participante não busca grupo/match posterior; sem grupo/match é erro |
| `number` | 0; /capture → /health | 0; mesmos paths | HTTPGo envia `{"value":"001"}`; Hurl envia bytes `{"value":001}`; sucesso não prova JSON equivalente |
| `boolean` | 0; /capture → /health | 0; mesmos paths | string `"true"` no HTTPGo, booleano `true` nos bytes Hurl |
| `json`, `placeholder` | 0; /capture → /health | 3; /capture | HTTPGo mantém string em valor JSON completo, inclusive placeholder sem recursão; Hurl não envia POST destes templates |
| `escapes` | 0; /capture → /health | 0; mesmos paths | aspas, barras, newline e Unicode escapados em string JSON |
| `static_defined`, `static_missing` | 0; /capture → /health | 3; /capture | padrão permanece literal no HTTPGo, com variável definida ou ausente; Hurl interpola |
| `latin1`, `utf8` | 0; /capture → /health | 0; mesmos paths | acentos, Unicode e multiline; charset explicitamente declarado |
| `invalid_utf8`, `unknown_charset` | 2; /capture | 3; /capture | erros textuais, nunca fallback/ausência; sem promessa geral de equivalência de charset |
| `header_first` | 0; /capture → /health | 3; /capture | primeira ocorrência `first,whole`, sem dividir vírgula; Hurl não fornece escalar neste header multivalorado |
| `header_second`, `header_missing` | 2; /capture | 3; /capture | segunda ocorrência não corrige a primeira; ausência não equivale a vazio |
| `invalid_late` | 2; zero envios | 3; /health → /capture | HTTPGo pré-compila todo arquivo; Hurl avalia padrão inválido depois dos envios |
| `unsupported_late` | 2; zero envios | 3; /health → /capture | lookahead com grupo rejeitado por ambos; HTTPGo rejeita antes da rede, Hurl após envios |
| `pipeline` | 2; zero envios | 0; /capture → /health | encadeamento arbitrário de filtros não suportado no recorte HTTPGo |
| `status_first` | 1; /capture | 4; /capture | divergência de status prevalece; não infere ordem interna Hurl pelo exit |
| `assertion_first` | 1; /capture | 3; /capture | HTTPGo assertion antes da captura; Hurl falha na captura |
| `collision` | 2; /capture | 0; /capture → /health | sem sobrescrita no HTTPGo; Hurl substitui inicial e envia `prenewpost` |
| `mixed` | 0; /capture → /health | 0; mesmos paths | HTTPGo envia regex `"42"` e JSONPath `42`; Hurl envia ambos `42` |

Primeiro grupo/correspondência e participação vazia são compatíveis nestes pares,
não uma promessa de equivalência de engines ou interpolação JSON. Diferenças de
primeira ocorrência, ordem, pré-validação, tipagem e colisão são deliberadas.
Segurança (grupo derivado, padrão sensível, captura pendente e lotes falhos),
isolamento, recursos e writers têm regressões próprias, não são homologados pela
segurança Hurl. Não há escolha de grupo, substituição, regex dinâmica ou assertions
da captura na própria resposta. YAML hierárquico/renderização externa continuam
**extensões HTTPGo**. Corpus sintético não homologa suíte real. Evidências em
[validation.md](validation.md#us-25--issue-48--história-0025).

## Inclusão e ausência — US-23 / issue #44

**Ampliação intencional do contrato:** JSON `contains`, `not contains` e
`not exists`; header `not contains`. Sem negação genérica ou JSONPath ampliado.
São **37 novos pares `membership_*`**, total **213 pares**. Os **176 anteriores
permanecem inalterados**, sem regeneração de goldens. O harness existente usa
`response_json` e `response_headers`, com fixtures independentes. Cada esperado
novo foi confrontado com **Hurl 8.0.0** em Linux, sem skip; códigos e paths reais
estão nos pares. Não se exige equivalência de stdout/diagnósticos ou códigos de
falha; 1/2 são categorias próprias HTTPGo, Hurl usa 4 nestas falhas de assertion.

| Par `membership_*` | HTTPGo | Hurl 8.0.0 | Classificação / observação |
|---|---|---|---|
| `demo` | 0; /capture → /health | 0; mesmos paths | inclui REQUIRED, exclui OTHER_TENANT/campo proibido e Domain |
| `substring`, `escape` | 0; /capture → /health | 0; mesmos paths | substring no JSON decodificado; newline, aspas e Unicode |
| `case`, `unicode_form` | 1; /capture | 4; /capture | sem normalizar caixa ou Unicode |
| `typed`, `nonrecursive` | 0; /capture → /health | 0; mesmos paths | escalares tipados, inclusive null; não busca dentro de elementos compostos |
| `numeric_string`, `forbidden` | 1; /capture | 4; /capture | número não equivale a string; presença proibida falha |
| `numbers`, `precision` | 0; /capture → /health | 0; mesmos paths | decimais equivalentes, zero com sinal; distingue 9007199254740993 de 9007199254740992 |
| `exponent` | 0; /capture → /health | 2; zero envios | gramática numérica HTTPGo aceita expoente enorme, sem expansão; não equivalência geral ao Hurl |
| `missing`, `index_missing` | 0; /capture → /health | 0; mesmos paths | ausência aprova negações; índice fora do array é ausência |
| `null_present`, `root_present` | 1; /capture | 4; /capture | null presente existe, inclusive raiz |
| `incompatible_null`, `incompatible_object`, `incompatible_number`, `incompatible_boolean`, `incompatible_string` | 1; /capture | 4; /capture | negação não aprova tipo incompatível, nem string com esperado numérico |
| `empty_array`, `empty_element` | 0; /capture → /health | 0; mesmos paths | array vazio exclui escalares; string vazia corresponde como elemento direto |
| `empty_substring` | 1; /capture | 4; /capture | string vazia não exclui substring vazia |
| `header_missing`, `header_empty` | 0; /capture → /health | 0; mesmos paths | ausência aprova, vazio presente exclui trecho não vazio |
| `header_empty_substring`, `header_comma` | 1; /capture | 4; /capture | vazio contém vazio; vírgula não divide valor único |
| `header_first` | 0; /capture → /health | 0; mesmos paths | sucesso coincidente: HTTPGo ignora Domain na segunda ocorrência; Hurl compara elementos inteiros da lista |
| `header_second` | 1; /capture | 0; /capture → /health | **diferença:** HTTPGo encontra substring Domain na primeira ocorrência; Hurl não encontra elemento inteiro igual a Domain na lista |
| `header_list_element` | 1; /capture | 4; /capture | HTTPGo encontra substring Domain=first na primeira; Hurl encontra esse elemento exato na lista |
| `header_first_order` | 1; /capture | 4; /capture | header divergente precede JSON inválido |
| `json_first_order` | 2; /capture | 4; /capture | JSON inválido anterior não é aprovação por negação; categoria própria de avaliação |
| `traversal` | 2; /capture | 0; /capture → /health | **diferença:** percurso por null é erro aqui, não ausência |
| `literal` | 0; /capture → /health | 4; /capture | **diferença:** esperado permanece literal aqui; Hurl interpola variável definida |
| `invalid_late` | 2; zero envios | 2; zero envios | operando em not exists rejeita todo arquivo |
| `generic_not` | 2; zero envios | 0; /health → /capture → /health | **não suportado:** header not exists não entra no recorte HTTPGo |

Primeira ocorrência de header é contrato preservado da US-07, não uma tentativa
de imitar as listas Hurl. Não concatenar Set-Cookie nem generalizar que todos os
predicados Hurl usam substring em headers repetidos. Números exatos, ausência e
tipos foram confrontados somente nas fronteiras acima. Durante a primeira
rodada, o Hurl rejeitou o esperado `1.23e2`: o par `numbers` passou a usar `123.0`
para isolar equivalência decimal, mantendo `exponent` como diferença explícita.
Somente novos pares foram ajustados após observar o binário; nenhuma regra de
negócio HTTPGo foi afrouxada. Não foi copiado/adaptado código-fonte Hurl.

Status → assertions na ordem escrita → capturas continua política própria;
proteção de operandos/callbacks, recursos, writers e isolamento têm testes
separados, rastreados em [validation.md](validation.md#us-23--issue-44--história-0023).
O corpus sintético não homologa a suíte real. YAML hierárquico e interpolação de
arquivos externos continuam **extensões HTTPGo**, não suporte Hurl acrescentado.

## Capturas literais de headers — US-22 / issue #42

**Ampliação intencional do contrato:** `[Captures]` aceita
`nome: header "nome-do-header"`, sempre string, primeira ocorrência e publicação
atômica após status/assertions. Os **18 pares `capture_header_*`** abaixo ampliam
o corpus para **176 pares**. Os **158 anteriores permanecem inalterados**;
nenhuma observação antiga foi regenerada. Novos goldens foram obtidos com
`-update` restrito aos novos pares, revisados e confirmados por `make compatibility`
sem update, Hurl **8.0.0** obrigatório e sem skip, em Linux.

O harness existente reutiliza `response_headers` e fixtures independentes. A
observação de requisição acrescenta somente `etag` e `if_none_match` opcionais;
`/catalog` responde 304 quando o `If-None-Match` corresponde ao ETag configurado.
Corpos e `X-Probe` fictícios já observados comprovam conteúdo/tipo, não apenas
exit codes. A tabela não exige igualdade de stdout, diagnósticos ou código
numérico de falha. Paths omitidos do segundo envio significam interrupção real.

| Par `capture_header_*` | HTTPGo | Hurl 8.0.0 | Classificação / dado observado |
|---|---|---|---|
| `cache` | 0; /catalog → /catalog | 0; mesmos paths | nome `eTaG`; ambos enviam `If-None-Match: "catalog-v1"` com aspas e aprovam 304 |
| `comma` | 0; /capture → /health | 0; mesmos paths | `X-Probe: first,whole`, sem dividir a vírgula |
| `empty_text` | 0; /capture → /health | 0; mesmos paths | captura vazia existe, composição textual envia `prepost` |
| `literal_text` | 0; /capture → /health | 0; mesmos paths | `X-Probe: {{missing}}` permanece literal, sem recursão |
| `missing` | 2; /capture | 3; /capture | ausência é erro de captura, sem próximo envio |
| `repeated` | 0; /capture → /health | 3; /capture | HTTPGo envia primeira ocorrência `first,whole`; Hurl não fornece escalar utilizável neste par multivalorado |
| `repeated_empty` | 0; /capture → /health | 3; /capture | primeira vazia não é substituída por `second`; HTTPGo envia `prepost` |
| `boolean` | 0; /capture → /health | 0; mesmos paths | JSON enviado é `{"value":"true"}` aqui e `{"value":true}` no Hurl |
| `null` | 0; /capture → /health | 0; mesmos paths | JSON enviado é `{"value":"null"}` aqui e `{"value":null}` no Hurl |
| `mixed` | 0; /capture → /health | 0; mesmos paths | lote header/JSONPath envia `{"header":"123","id":42}` aqui; Hurl envia `{"header":123,"id":42}` |
| `strings` | 0; /capture → /health | 3; /capture | strings numérica/booleana/semelhante a objeto preservadas aqui; Hurl não envia o POST deste template JSON |
| `empty` | 0; /capture → /health | 3; /capture | aqui JSON com string vazia e texto composto; Hurl falha com placeholder vazio em valor JSON sem aspas, não prova falha de captura vazia |
| `literal` | 0; /capture → /health | 3; /capture | aqui placeholder recebido vira string JSON; Hurl não envia este POST com placeholder JSON não entre aspas; comparar `literal_text` separadamente |
| `collision` | 2; /capture | 0; /capture → /health | HTTPGo mantém inicial `x=original`, sem sobrescrita; Hurl envia `X-Probe: new` |
| `assertion_first` | 1; /capture | 3; /capture | HTTPGo assertion antes da captura ausente; Hurl erro de captura |
| `status_first` | 1; /capture | 4; /capture | ambos interrompem na divergência de status; exit não prova se Hurl avaliou captura ausente |
| `invalid_late` | 2; zero envios | 2; zero envios | header sem aspas na entrada tardia rejeita o arquivo inteiro |
| `filter` | **0; /capture → /health (US-25)** | 0; /capture → /health | contrato ampliado na US-25: ambos enviam `X-Probe: literal`; era não suportado na US-22 |

São diferenças deliberadas de **primeira ocorrência**, **interpolação JSON
contextual**, **ordem** e **não sobrescrita**, não motivos para alargar o recorte
ou remover verificações. Um sucesso de ambos com bytes JSON diferentes não é
equivalência semântica. Capturas de header não ampliam o JSONPath suportado.

Proteção automática de headers sensíveis e sua proveniência são contrato próprio
HTTPGo, comprovado por testes de runner/CLI/E2E, não por equivalência de segurança
com Hurl. Não há cookie jar nem extração implícita de cookie/atributos (extração explícita via regex é US-25); Hurl é executado com
`--no-cookie-store`. O corpus sintético não homologa uma suíte real. Recursos,
redirect final, falhas de consumo/saída e isolamento são rastreados em
[validation.md](validation.md#us-22--issue-42--história-0022).

## Ampliação US-21 — busca textual no body (issue #40)

**Ampliação intencional do contrato:** `body contains "trecho"` e
`body not contains "trecho"` deixam de ser rejeitados em `[Asserts]`, exigindo
`HTTP` e esperado literal. Regex e outros operadores de body continuam não
suportados. São **19 novos pares `body_*`**, total **158**; os **139 pares
anteriores permanecem inalterados**, sem regeneração de observações.

O harness ganhou `response_body` opcional (base64 JSON de bytes), distinguindo
configuração ausente de body vazio, preservando `response_json`, `Content-Type`
explícito e sessões/árvores de fixtures independentes entre executáveis.
A tabela registra execução real em Linux contra **Hurl 8.0.0**; paths são os
mesmos em ambos, salvo menção contrária. Não compara texto de mensagens ou afirma
compatibilidade de charsets não confrontados.

| Par `body_*` | HTTPGo | Hurl 8.0.0 | Classificação / observação |
|---|---|---|---|
| `demo` | 0; /capture → /health | 0; mesmos paths | compartilhado: presença e ausência no body final |
| `forbidden` | 1; /capture | 4; mesmo path | condição falsa interrompe; códigos próprios |
| `empty_contains`, `empty_absent` | 0; /capture → /health | 0; mesmos paths | vazio contém `""` e exclui texto não vazio |
| `empty_not_contains`, `empty_missing` | 1; /capture | 4; mesmo path | vazio não exclui `""` nem contém texto não vazio |
| `case`, `spaces`, `unicode_form` | 1; /capture | 4; mesmo path | sem normalização de caixa, espaços ou Unicode |
| `newline` | 0; /capture → /health | 0; mesmos paths | quebra de linha literal preservada |
| `json_original` | 0; /capture → /health | 0; mesmos paths | busca escape recebido, não JSON reserializado |
| `utf8`, `utf8_declared` | 0; /capture → /health | 0; mesmos paths | UTF-8 omitido/declarado, incluindo U+FFFD originalmente válido |
| `latin1` | 0; /capture → /health | 0; mesmos paths | rótulo ISO-8859-1 usa windows-1252, incluindo byte 0x80 → € |
| `utf16le` | 0; /capture → /health | 0; mesmos paths | UTF-16LE com U+FFFD válido |
| `gb18030_replacement` | 0; /capture → /health | 0; mesmos paths | U+FFFD válido ao lado de bytes alternativos válidos de GB18030; não exige reencodificar os demais caracteres |
| `invalid_utf8`, `truncated_utf16`, `unknown_charset` | 2; /capture | 4; mesmo path | erro textual controlado aqui; bytes ilegíveis não provam ausência, nem em body vazio |

**Diferenças e limites explícitos:** códigos 0/1/2 são do HTTPGo; falha do Hurl
nesses pares é 4. Esperados HTTPGo nunca interpolam; não há opt-in nessa entrega.
Status → assertions na ordem escrita → capturas continua sendo a política local,
sem prometer a ordem de captura do Hurl. Rótulos resolvidos pelo índice web de
`golang.org/x/text v0.44.0`, sem sniffing HTML, fallback após erro ou normalização.
O corpus prova apenas os rótulos/conversões listados, não todos os encodings.
Busca literal não garante detectar representações alternativas do mesmo segredo.
Segurança de callback/coleção/streams, cancelamento, leitura/fechamento, erro de
saída e cenário imutável têm regressões próprias `TestBody*`, sem expor payloads
sensíveis no diferencial. YAML e renderização de arquivos continuam extensões
HTTPGo; esta história não adiciona extensão de gramática para body.

## Contrato de suíte — US-16 (issue #38)

Descoberta de diretórios, normalização/deduplicação, ordenação por caminho relativo
ao cwd com `/`, links, configuração YAML e `--fail-fast` pertencem ao contrato da
CLI HTTPGo. Não são novas construções da linguagem `.hurl` nem promessa de
semântica idêntica à CLI Hurl. São verificados separadamente em
`internal/suite`, `internal/cli/suite*_test.go` e `tests/e2e/suite*_test.go`.
Os fixtures próprios estão em `testdata/suites/basic/`, fora do corpus diferencial.

Caminhos exibidos passam a relativos normalizados (fallback absoluto entre
unidades); resumos por arquivo preservam contadores e acrescentam estado/duração,
e `resumo suíte:` agrega inclusive arquivo único. Arquivo sem parsing completo
não recebe contagem planejada inventada. Exit codes próprios continuam 0/1/2,
com 2 prevalecendo sobre 1; itens não executados nunca são aprovados. Configuração
única, sessão/capturas novas por arquivo e ausência de cookie jar continuam
restrições explícitas. A proteção de variantes persiste entre arquivos da mesma
invocação sem compartilhar variáveis capturadas. Não anuncia JUnit, paralelismo,
dependências entre arquivos nem login compartilhado.

Não há modificação do corpus de **139 pares** nem regeneração das observações
anteriores nesta história. O gate permanece `make compatibility` com binário
**Hurl 8.0.0**, sem skip; evidência efetiva em `validation.md`. A ampliação da
CLI não amplia gramática, consultas ou predicados suportados.

## Ampliação US-15 — padrões textuais (issue #36)

Foram adicionados três pares `matches_*`, executados pelo harness contra **Hurl 8.0.0** com `response_json` e servidor local. A evidência é limitada às strings/padrões da tabela; não afirma equivalência geral de sintaxe regex, escapes JSON ou mensagens.

| Par | HTTPGo | Hurl 8.0.0 | Observação em Linux |
|---|---|---|---|
| `matches_prefix` | 0; /capture | 0; /capture | `^Pizza` corresponde ao início de `Pizza Margherita` |
| `matches_substring` | 0; /capture | 0; /capture | `Pizza` sem âncoras corresponde a trecho em `Mini Pizza` |
| `matches_anchored` | 1; /capture | 4; /capture | `^Pizza$` não corresponde a `Mini Pizza`; códigos de falha são próprios |

As observações estão versionadas nos pares `.hurl`/`.json`; resposta é JSON literal fornecida pelo fixture do harness. Nenhum código-fonte Hurl foi utilizado. Construções Go fora de `regexp/syntax` (incluindo lookaround e backreferences) são rejeitadas pelo HTTPGo; os pares acima não verificam equivalência desses recursos nem de escapes. Esperados permanecem literais em ambas as ferramentas nesta evidência. A divergência de código não é corrigida para imitar Hurl. A US-24 acrescenta `matches` em headers (seção própria acima); body/variáveis continuam não suportados.

## Ampliação US-14 — estrutura, quantidade e limites (issue #34)

**Ampliação intencional do contrato:** `exists` JSON, `count` de arrays e os
comparadores `>`, `>=`, `<`, `<=` deixam de ser rejeitados. No corpus de cenários,
as antigas rejeições globais de exists/count passam a `!=`/`length`, que continuam
não suportados. Novos negativos documentam esperados inválidos e count em captura.
São **20 novos pares `structure_*`**, total **136**; as **116 observações antigas
permanecem inalteradas**. Sem reutilizar código-fonte Hurl, alterar o harness ou
regenerar observações anteriores.

Todos os pares abaixo foram executados contra o binário **Hurl 8.0.0**. Presença
de **null** foi confrontada antes de ser classificada como compartilhada. Os pares
são evidência específica, não equivalência geral de números/JSONPath ou diagnósticos.

| Par `structure_*` | HTTPGo | Hurl 8.0.0 | Observação em Linux |
|---|---|---|---|
| `presence` | 0; /capture → /health | 0; mesmos paths | raiz, null, string/array/objeto vazios, false e zero presentes existem |
| `missing`, `index_missing` | 1; só /capture | 4; mesmo path | ausência difere de null; interrupção compartilhada, códigos próprios |
| `count_operators` | 0; /capture → /health | 0; mesmos paths | cinco comparadores, array vazio e elementos null |
| `count_mismatch` | 1; só /capture | 4; mesmo path | tamanho divergente impede próximo envio |
| `count_string`, `count_object`, `count_null` | 1; só /capture | 4; mesmo path | tipo incompatível falha; não se conta implicitamente string/objeto aqui |
| `count_missing` | 1; só /capture | 4; mesmo path | ausência não é array vazio |
| `limits` | 0; /capture → /health | 0; mesmos paths | quatro comparadores, negativos/decimais, limites inclusivos e zero com sinal |
| `gt_boundary`, `ge_boundary`, `lt_boundary`, `le_boundary` | 1; só /capture | 4; mesmo path | fronteiras falsas, códigos próprios |
| `numeric_string` | 1; só /capture | 4; mesmo path | string numérica não passa por coerção |
| `precision` | 0; /capture → /health | 0; mesmos paths | 9007199254740993 > 9007199254740992 neste par |
| `exponent` | 0; /capture → /health | 2; zero contatos | diferença de gramática: esperado com expoente enorme é aceito exatamente aqui |
| `count_huge` | 0; /capture → /health | 0; mesmos paths | quantidade esperada além do inteiro de máquina é aceita neste par por ambos |
| `count_negative`, `count_fractional` | 2; zero contatos | 4; só /capture | restrição intencional: HTTPGo pré-valida quantidade inteira não negativa; Hurl chega à assertion |

A primeira rodada revelou que o Hurl aceita a quantidade enorme de `count_huge`;
somente esse novo esperado foi corrigido conforme a observação, sem mudar a
semântica aprovada. A proteção de valores conhecidos/derivados, ocultação integral
de contagens com captura secreta pendente e diagnóstico sem coleção são testados
separadamente nos testes próprios de runner/CLI/E2E. O diferencial usa só dados
fictícios e não compara texto de mensagens. Esperados continuam literais, count
em capturas/strings/objetos continua fora do suporte, assim como filtros, agregações,
aritmética, regex e ampliação da linguagem de consulta.

## Extensão US-13 — referências ao ambiente do processo (issue #32)

`${NOME}` como folha string completa de `variables` em YAML base/ambiente é
**extensão HTTPGo**, não construção `.hurl` nem promessa de equivalência de
configuração/segredos com Hurl. Nomes ASCII `[A-Za-z_][A-Za-z0-9_]*`; sem composição
parcial ou referências em listas. Valida cada fonte, aplica base < ambiente < CLI,
consulta somente referências efetivas e obrigatórias. Ausente falha com 2 antes de
HTTP; vazio presente e literais numéricos/boolean/null permanecem strings. Conteúdo
recebido não é expandido novamente. Destinos são marcados antes de overrides,
acumulando `secrets`/`--secret`; protege valores conhecidos, substitutos e derivados
nos resultados/assertions/callbacks/streams. Credenciais reais em flags continuam
proibidas. Sem dotenv, stdin, overrides `HTTPGO_*`, cofres ou JUnit.

`TestProcess*` em config, CLI, runner e E2E comprova o contrato separadamente,
com consulta falsa contada e HTTP real no binário, ambiente de filho controlado,
strings/escaping, ausência/vazio, precedência, rejeições antes de rede, eco de
segredo e falhas precoces. Fixtures positivas/negativas em `testdata/config` e
cenário `scenario_process_profile.hurl`. Os **116 pares existentes** de
`testdata/compatibility/` permanecem inalterados: sem regenerar observações,
passar YAML/flags próprias ao Hurl ou copiar/adaptar fontes Hurl. O diferencial
continua exigindo **Hurl 8.0.0**, sem skip.

## Extensão US-12 — ambientes nomeados (issue #29)

`--env <nome>` seleciona somente `environments/<nome>.yaml` relativo à base
selecionada, mesmo vazia (sem base, cwd). É **extensão HTTPGo**, não sintaxe
Hurl nem equivalência com ambientes da outra ferramenta. Versão 1 opcional no
ambiente; validação YAML reutilizada da US-11. Precedência base < ambiente < CLI,
merge tipado por caminho e união das marcas de segredos. `${NOME}` segue a
extensão US-13 acima; overrides `HTTPGO_*`, stdin e composição de vários ambientes
continuam fora do suporte.

Os testes `TestEnvironment*` exercitam seleção, fontes inválidas, tipos, prazo,
segurança, isolamento e modos inline/externo/multipart pelo binário real. Exemplos
positivos/negativos YAML ficam em `testdata/config/environments/`, separados do
corpus Hurl. O `.hurl` demonstrativo é `scenario_environment_products.hurl`.
Sem alteração de construções compartilhadas: **116 pares existentes inalterados**,
sem regenerar observações nem fornecer `--env`, `--config` ou `--render-files` ao
Hurl 8.0.0. O diferencial continua verificando somente o subconjunto anterior.

## Extensão US-11 — configuração YAML (issue #27)

`--config` e descoberta de `.httpgo.yaml` são **extensões HTTPGo**, não sintaxe
Hurl nem promessa de equivalência com arquivos de configuração Hurl. Não mudam
as construções `.hurl` compartilhadas. O corpus YAML está separado em
`testdata/config/`; o exemplo `.hurl` está em
`testdata/scenarios/scenario_config_products.hurl`.

Testes `TestConfig*` unitários, HTTP/TLS e de binário comprovam carga segura,
composição/tipos, precedência, prazo e proteção inicial/dinâmica. Configuração
fora do cwd não altera origem dos payloads e arquivos multipart permanecem
literais. Timeout de produção agora pode ser diferente dos dez segundos padrão;
o diferencial continua usando os defaults anteriores em ambas as ferramentas.

Os **116 pares existentes** de `testdata/compatibility/` permanecem inalterados.
Não se fornece `--config`/`--render-files` ao Hurl e não se regeneram observações
para forçar equivalência. Stdin e sandbox continuam fora do suporte; ambientes
nomeados e referências de processo têm os contratos US-12/US-13 separados acima.
YAML inválido, recursos YAML proibidos e referências parciais/aninhadas são
rejeições explícitas antes de qualquer HTTP, verificadas nos testes próprios.


## Extensão US-10 — JSON externo opt-in (issue #25)

`--render-files` é uma **extensão HTTPGo**, não um recurso declarado compatível
com Hurl. Trata todos os arquivos usados como body completo como templates JSON,
sem seleção por extensão/Content-Type. Reutiliza JSON contextual tipado, sessão,
variáveis iniciais/capturas anteriores e proteção de segredos; arquivos multipart
continuam literais. Sem flag, os bytes da US-09 permanecem inalterados.

Os testes da extensão ficam em `render_files*_test.go` de runner/CLI/transporte e
E2E, **separados** do diferencial. Comprovam tipos/escapes, JSON estático exato,
validação tardia, origem original segura, modos mistos e políticas HTTP. Não se
passa `--render-files` ao Hurl nem se alteram observações para forçar equivalência.
Os **116 pares existentes** continuam exercitando o modo bytes e o subconjunto
anterior, sem mudanças em `testdata/compatibility/`. A nova capacidade não implica
renderização externa equivalente pelo Hurl. YAML tem agora o contrato US-11
separado acima; sandbox continua fora do suporte.

## Ampliação US-20 — multipart (issue #24)

**Mudança intencional de contrato:** `[MultipartFormData]` deixa de ser globalmente
rejeitado. O negativo antigo foi renomeado para `unsupported_multipart_get.hurl`;
GET/DELETE, duplicações e formulários fora do recorte continuam rejeitados.
São **116 pares**, dez novos `multipart_*`; as **106 observações anteriores
não mudaram**. Nenhum código-fonte Hurl foi usado.

O harness registra partes **em ordem**, nome, filename bruto de Content-Disposition
(sem normalizá-lo com `Part.FileName`), MIME e bytes (`data` em base64 no JSON,
inclusive binário/vazio). Não compara framing bruto nem boundary aleatório.
Content-Type válido é normalizado a `multipart/form-data` após usar o boundary
real para ler as partes. Header com boundary duplicado é marcado como inválido;
nesse caso o framing do último boundary gerado é observado separadamente, sem
anunciar que o header inválido seria utilizável por qualquer servidor.

| Par | HTTPGo | Hurl 8.0.0 | Evidência / diferença em Linux |
|---|---|---|---|
| `multipart_post` | 0; POST /documents | 0; mesmo contato | campos estáticos/interpolados, MIME e bytes compartilhados |
| `multipart_put` | 0; PUT /documents | 0; mesmo contato | arquivo sem textos, Content-Type declarado sem boundary |
| `multipart_patch` | 0; PATCH /documents | 0; mesmo contato | texto antes/depois do arquivo; ordem preservada |
| `multipart_empty` | 0; POST /documents | 0; mesmo contato | zero bytes e inferência `.txt` → `text/plain` em ambos |
| `multipart_binary` | 0; POST /documents | 0; mesmo contato | NUL/controle, CRLF e placeholder indefinido intactos; octet-stream |
| `multipart_unknown` | 0; POST /documents | 0; mesmo contato | extensão desconhecida usa `application/octet-stream` |
| `multipart_spaces` | 0; POST /documents | 2; zero contatos | caminho literal com espaço/Unicode aceito aqui, rejeitado pela gramática Hurl deste par |
| `multipart_duplicate` | 2; zero contatos | 0; POST /documents | HTTPGo não aceita nomes repetidos |
| `multipart_multiple` | 2; zero contatos | 0; POST /documents | HTTPGo exige exatamente um arquivo |
| `multipart_boundary` | 2; zero contatos | 0; POST /documents | Hurl/libcurl envia dois parâmetros boundary, um manual e um gerado; HTTPGo proíbe manual |

**Filename difere intencionalmente:** nos pares aceitos por ambos, Hurl envia
`payloads/raw.txt`, `payloads/empty.txt` etc. como filename bruto; HTTPGo envia só
`raw.txt`, `empty.txt` etc., sem caminho local, conforme RN-04. Não se usa a
normalização de basename do receptor para esconder essa diferença. Os testes
próprios comprovam espaços/Unicode e referências absolutas/fora da raiz, sem
prometer igualdade de gramática Hurl. Inferência em outras extensões segue a tabela
HTTPGo documentada em `subset.md`, não se deduz igualdade de bancos MIME do SO.

Continuam fora do recorte: multipart avançado, campos repetidos, arquivo renderizado,
filename customizado, headers de parte e sandbox. A flag `--render-files` da
US-10 não altera esse contrato nem renderiza arquivos multipart.

## Ampliação US-09 — payload externo literal (issue #22)

**Mudança de contrato:** `file,...;` passa de rejeitado a suportado para métodos
com body. O negativo global `unsupported_external_body.hurl` vira
`unsupported_get_external_body.hurl`; novos positivos/negativos detalham o recorte.
São **106 pares**, seis novos `external_*`; as 100 observações anteriores não mudam.
O harness copia `payloads/` junto de cada cenário temporário, em árvores isoladas
para httpgo e Hurl. As referências relativas do corpus não viram caminhos
absolutos artificiais. Cada ferramenta resolve os arquivos a partir de seu cenário.
O corpo registrado é exatamente o recebido pelo servidor, não uma leitura presumida.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/classificação observada em Linux |
|---|---|---|---|
| `external_relative` | 0; POST /health | 0; mesmo contato | bytes JSON incluindo espaços/LF e Content-Type explícito idênticos |
| `external_empty` | 0; POST /health | 0; mesmo contato | body vazio, sem Content-Type inferido neste par |
| `external_literal` | 0; POST /health | 0; mesmo contato | não JSON com whitespace/LF e `{{placeholder}}` literal sem variável |
| `external_missing` | 2; somente GET /health | 3; somente GET /health | falha na vez da segunda entrada, sem enviar segunda/terceira; códigos distintos |
| `external_directory` | 2; nenhum contato | 3; nenhum contato | diretório não é body legível; códigos distintos |
| `external_spaces` | 0; POST /health, bytes `not JSON` + LF | 2; nenhum contato | gramática deliberadamente diferente: nome literal com espaço aceito aqui, rejeitado neste par Hurl |

Não se deduz igualdade geral da gramática de caminhos, aspas, escapes,
interpolação de nomes ou códigos. Os testes próprios cobrem absolutos, `..` fora
do projeto, nomes com espaços/placeholders e diretórios correntes distintos.
Acesso negado real é verificado pelo E2E Linux sob usuário sem privilégios;
o diferencial acima observa inexistente/diretório, não anuncia equivalência de
mensagem de permissão. Não há sandbox automática, renderização externa,
`--render-files`, multipart ou inferência de MIME **na entrega US-09** (multipart
agora tem o contrato separado acima). A US-10 implementa renderização externa
como extensão HTTPGo separada, sem mudar o diferencial do modo bytes.

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
--max-time 10 --retry 0 --no-cookie-store --no-proxy '*'` nos pares legados;
`polling_*` usa a opção local, com a exceção explícita `cli_zero` acima.
Não se compara texto
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
seções além de `[Captures]`, `[Asserts]` e `[MultipartFormData]`,
bodies inline binários e multipart avançado, headers de enquadramento e
demais construções fora de escopo continuam explicitamente não suportadas.

## Ampliação US-05 — capturas string (issue #14)

Nove pares `captures_*` ampliam o contrato para **50 pares**. Os **41 pares
anteriores não mudaram**. No corpus de cenários, a antiga rejeição global
`unsupported_section_captures.hurl` virou `invalid_capture_empty.hurl`: a seção
agora é suportada, mas vazia continua inválida. Negativos específicos mantêm
wildcard de objeto (array `[*]` passa a suportado na US-27), filtro, descida recursiva, índice negativo e outras queries rejeitados
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
| `captures_non_string` | **0 na US-06**; /capture e /health, X-Probe `123` | 0; mesmos requests | antiga restrição string removida intencionalmente na issue #16 |
| `captures_unsupported` | **0 na US-27**; /health, login e /health | 0; mesmos paths | restrição histórica de wildcard de array removida intencionalmente na issue #52 |
| `captures_mismatch` | 1; somente /unavailable | 4; mesmo path | status divergente prevalece sobre captura de body não JSON |

Os testes próprios httpgo cobrem também strings semelhantes a placeholders,
controles/Unicode/escapes, URL/headers inválidos após captura, múltiplas capturas,
conflitos hierárquicos e de lote, isolamento de execução e proteção dinâmica.
Não se deduz compatibilidade geral de JSONPath/capturas a partir desses pares.

## Ampliação US-06 — capturas tipadas (issue #16)

**Mudança intencional de contrato:** `captures_non_string.json` passa de exit 2
com somente `/capture` para exit 0 com `/capture` → `/health`, header `X-Probe: 123`.
O `.hurl` não mudou. A captura numérica usa representação JSON em contexto textual;
isso não é conversão implícita de estruturas para strings. **Os outros 49 pares
antigos não mudaram.** Oito pares novos ampliam a matriz para **58 pares**,
reutilizando `response_json` e as observações de URI, headers e bytes do body.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença observada |
|---|---|---|---|
| `captures_typed_precision` | 0; ID `9007199254740993` em URI/header/JSON numérico | 0; mesmos bytes | precisão exata compartilhada neste par, sem extrapolar outros números |
| `captures_typed_boolean` | 0; `true` raiz, JSON booleano e texto em URI/header/string | 0; mesmos bytes | representação escalar contextual compartilhada neste par |
| `captures_typed_null` | 0; null raiz, JSON null e texto `null` em URI/header/string | 0; mesmos bytes | null encontrado não é ausência neste par |
| `captures_typed_values` | 0; booleano, null, string/array/objeto vazios preservados | 3; somente /capture | Hurl não envia a segunda entrada deste conjunto; não se deduz qual tipo isolado causou a falha |
| `captures_typed_root` | 0; objeto raiz com `9007199254740993`, `1.2300e+99`, null e placeholder literal | 3; somente /capture | HTTPGo preserva números aninhados e conteúdo literal; sem equivalência composta presumida |
| `captures_typed_index` | 0; `$[0].items` produz array raiz com tipos e precisão | 3; somente /capture | seleção por índice/propriedade e inserção de array completo comprovadas aqui |
| `captures_typed_text_rejected` | 2; somente /capture | 3; somente /capture | HTTPGo exige valor JSON completo para array, sem coerção para string JSON |
| `captures_typed_header_rejected` | 2; somente /capture | 3; somente /capture | HTTPGo rejeita objeto em header antes do envio dependente |

Essas observações não implicam equivalência geral de tipos JSON, JSONPath,
precisão ou conversão textual com Hurl. Os pares de string anteriores permanecem:
`captures_literal` ainda conserva `"null"` como string aqui enquanto o Hurl envia
null JSON. Nenhuma observação antiga foi atualizada para esconder divergências.
O teste diferencial não avalia proteção de credenciais: usa só valores fictícios;
E2E próprios cobrem capturas secretas escalares/compostas, constituintes, escapes
conhecidos, falhas posteriores e ausência de mutação dos dados enviados.

## Ampliação US-07 — assertions de headers e HTTP * (issue #18)

**Ampliação intencional do contrato antes rejeitado:** `[Asserts]` agora aceita
`header` com `==`, `contains` e `exists`; `HTTP *` aceita qualquer status recebido.
A rejeição global `unsupported_section_asserts.hurl` virou
`invalid_asserts_without_http.hurl`, acompanhada de negativos específicos de query,
operador, filtro e sintaxe. Isso deve ser sinalizado na revisão/PR.

**21 pares novos**, total **79**. Os **58 pares anteriores permanecem inalterados**.
O harness ganhou `response_headers`: lista ordenada de nome/valor fictícios,
permitindo ocorrências repetidas e valores vazios, sem perder dados por mapa.
`net/http` canonicaliza os nomes ao escrever; a ordem das ocorrências é preservada.
Caixas diferentes no wire são exercitadas adicionalmente no transporte próprio.
As observações novas foram escritas/revisadas individualmente e confirmadas com
`make compatibility`, sem atualização automática dos pares antigos.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença observada |
|---|---|---|---|
| `headers_operators` | 0; /health → /missing | 0; mesmos paths | três operadores e consulta de nomes sem distinção de caixa compartilhados, com uma ocorrência |
| `headers_equal_mismatch`, `headers_contains_mismatch` | 1; somente /health | 4; somente /health | valores case-sensitive, igualdade/substring literal e interrupção compartilhados; códigos próprios |
| `headers_exists_missing` | 1; somente /health | 4; somente /health | ausência falha em exists mesmo com `HTTP *` |
| `headers_empty` | 0; /health → /missing | 0; mesmos paths | vazio existe e é igual a `""`; substring vazia passa em valores presentes vazios/não vazios |
| `headers_missing_empty`, `headers_missing_contains_empty` | 1; somente /health | 4; somente /health | ausência difere de vazio e não contém substring vazia |
| `headers_repeated_first` | 0; /health → /missing | 4; somente /health | **divergência deliberada**: primeira ocorrência aqui; Hurl retorna lista e falha em igualdade com string e substring escalar |
| `headers_repeated_second` | 1; somente /health | 4; somente /health | ambos falham, mas **não comprova equivalência**: primeiro valor aqui, lista no Hurl |
| `headers_repeated_exists` | 0; /health → /missing | 0; mesmos paths | presença de header repetido, inclusive com primeiro valor vazio |
| `headers_comma_literal` | 0; /health → /missing | 0; mesmos paths | uma ocorrência com vírgula permanece string única na igualdade e substring |
| `headers_comma` | 1; somente /health | 4; somente /health | uma ocorrência não é dividida por vírgula para igualdade |
| `headers_wildcard`, `headers_wildcard_alone` | 0; /health → /missing → /unavailable | 0; mesmos paths | `HTTP *` aceita 200/404/503, com e sem assertions; resumo próprio não conta comparação numérica |
| `headers_without_http`, `headers_invalid_late` | 2; zero envios | 2; zero envios | exigência de HTTP para `[Asserts]` e sintaxe inválida tardia rejeitam arquivo integral |
| `headers_query_unsupported` | **0 na US-08**; /capture → /health | 0; mesmos paths | antiga rejeição de igualdade JSON removida intencionalmente na issue #20 |
| `headers_operator_unsupported` | 2; zero envios | 0; /health → /missing | restrição explícita: `!=` fora do subconjunto |
| `headers_literal_missing`, `headers_literal_defined` | 0; /health → /missing | 4; somente /health | esperado `{{name}}` permanece literal aqui; Hurl tenta resolver variável (ausente: Undefined variable; definida: divergência) |
| `headers_capture_precedence` | 1; somente /capture | 3; somente /capture | **divergência deliberada**: headers falham antes de examinar JSON/capturas aqui; Hurl falha primeiro na captura |

Não se anuncia compatibilidade de comparação escalar com headers repetidos nem
interpolação de esperado. A regra da história de primeira ocorrência e precedência
não foi alterada para forçar equivalência. Segurança de diagnósticos, prioridade de
escrita, ausência de retenção do body em headers isolados sem capturas/JSON, redirects finais, timeout e
cancelamento são verificados nos testes próprios; o diferencial não exige mensagens
iguais nem generaliza equivalência entre net/http e libcurl.

## Ampliação US-08 — igualdade JSON (issue #20)

**Mudança intencional de contrato:** `headers_query_unsupported.json` passa de
exit 2/zero envios para exit 0 com `/capture` → `/health`. O `.hurl` não muda: era
uma igualdade JSON válida, agora implementada. A antiga rejeição global no corpus
de cenários passou historicamente a wildcard não suportado (agora wildcard de objeto, após US-27), acompanhada de
negativos específicos de literal, operador, filtro e consulta dinâmica.
**Os outros 78 pares antigos não mudam.** São **21 pares novos**, total **100**,
reutilizando o harness `response_json` e `response_headers`, sem dependência nova
nem atualização automática indiscriminada das observações.

| Corpus novo | httpgo | Hurl 8.0.0 | Evidência/diferença observada |
|---|---|---|---|
| `assert_json_scalars` | 0; /capture → /health | 0; mesmos paths | string/número/booleano, string vazia, false e null presente compartilhados neste par |
| `assert_json_type` | 1; somente /capture | 4; somente /capture | string `"4500"` difere de número 4500; interrupção compartilhada, códigos próprios |
| `assert_json_decimals` | 0; /capture → /health | 0; mesmos paths | esperado decimal, resposta com expoente e zero com sinal comparam por valor neste par |
| `assert_json_numbers` | 0; /capture → /health | 2; zero envios | **diferença de gramática**: Hurl rejeita o literal esperado `125e-2` antes da rede; HTTPGo compara expoentes exatamente |
| `assert_json_precision` | 1; somente /capture | 4; somente /capture | seleção `$.items[0].id`: 9007199254740993 difere de 9007199254740992, sem arredondamento neste par |
| `assert_json_object` | 0; /capture → /health | 2; zero envios | igualdade estrutural ignora ordem das chaves aqui; Hurl rejeita literal esperado objeto |
| `assert_json_array_order` | 1; somente /capture | 2; zero envios | ordem de arrays importa aqui; Hurl rejeita literal esperado array |
| `assert_json_empty_structures` | 0; /capture → /health | 2; zero envios | arrays/objetos vazios aceitos aqui; literal composto rejeitado no Hurl |
| `assert_json_nested_precision` | 1; somente /capture | 2; zero envios | diferença numérica aninhada preservada aqui; literal objeto rejeitado no Hurl, sem equivalência presumida |
| `assert_json_missing`, `assert_json_index_missing` | 1; somente /capture | 4; somente /capture | propriedade/índice ausentes falham contra null; não se deduz equivalência geral das categorias de avaliação |
| `assert_json_incompatible` | 2; somente /capture | 4; somente /capture | percurso por número é erro de avaliação aqui, distinto de ausência; ambos interrompem |
| `assert_json_invalid_body` | 2; somente /capture | 4; somente /capture | resposta JSON inválida é erro de avaliação aqui, não transporte |
| `assert_json_header_first` | 1; somente /capture | 4; somente /capture | header divergente anterior prevalece sobre body inválido aqui; não compara texto/categoria das mensagens do Hurl |
| `assert_json_json_first` | 2; somente /capture | 4; somente /capture | invertendo a ordem, JSON inválido prevalece aqui; códigos/categorias próprios |
| `assert_json_capture_precedence` | 1; somente /capture | 3; somente /capture | **divergência deliberada**: assertion JSON falha antes de captura ausente aqui; Hurl avalia captura primeiro |
| `assert_json_literal` | 0; /capture → /health | 4; somente /capture | esperado `{{name}}` é literal aqui, mesmo definido; Hurl interpola |
| `assert_json_invalid_late`, `assert_json_without_http` | 2; zero envios | 2; zero envios | literal malformado/ausência de HTTP em entrada tardia rejeita arquivo integral |
| `assert_json_query_unsupported` | **0 na US-27**; /health → /capture | 0; mesmos paths | wildcard de array passa a suportado intencionalmente na issue #52 |
| `assert_json_operator_unsupported` | 2; zero envios | 0; /capture → /health | `!=` continua fora do recorte aqui |

A primeira rodada encontrou a rejeição do esperado com expoente e a aceitação
do wildcard no Hurl; somente as duas novas observações foram corrigidas e os
100 pares foram reexecutados sem `-update`. Não se alterou a semântica aprovada
para imitar a referência. Valores esperados compostos/expoentes, literais sem
interpolação e classificações 0/1/2 são o contrato HTTPGo; não se anuncia
compatibilidade Hurl geral dessas combinações nem dos motores HTTP.

Evidência própria adicional cobre expoentes muito grandes sem expansão de memória,
comparação recursiva exata, seis tipos na raiz/propriedades/índices, escapes,
strings literais aninhadas, ordem de falhas, proteção anterior ao callback e
captura secreta pendente. Neste último caso **ambos** os operandos de falhas de
headers/JSON são integralmente ocultos sem avaliar a captura antecipadamente,
inclusive quando ela falharia. O diferencial usa dados fictícios e não é evidência
de proteção de credenciais: isso é exercitado por runner/CLI/E2E próprios.

## Extensões e restrições

Interpretação tipada de literais JSON na CLI é uma extensão implementada na US-04,
com mudança intencional do contrato textual temporário da US-03. Não há conversão
implícita de objetos/arrays em texto nem navegação de propriedades.

Interpolação JSON em arquivos externos com `--render-files` é extensão HTTPGo
implementada na US-10, **não parte do subconjunto Hurl**. YAML hierárquico é a
extensão US-11, implementada/testada separadamente conforme a seção inicial.
A gramática menor, saída/exit codes próprios
e restrição de redirects estão detalhados em `subset.md`.

## Ambiente usado

A execução local usou Hurl 8.0.0 com libcurl 8.18.0. O binário da sandbox emite um
aviso de versionamento de libcurl em stderr ao iniciar; o harness não o confunde
com falha de HTTP ou altera os resultados para escondê-lo. A matriz acima passou
contra esse binário. Evidências Linux por história em `validation.md`. Conforme o
plano v4, seção 7, builds cruzados e runtime macOS/Windows ficam opcionais nesta
história, para validação final de portabilidade/release; não bloqueiam seu gate.
Nenhuma execução nativa nessas plataformas foi declarada.
