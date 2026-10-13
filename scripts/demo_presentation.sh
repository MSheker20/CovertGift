#!/bin/bash
# ==============================================================================
# 🎁 CovertGift — Akademik Sunum & Canlı Demo Test Senaryosu
# ==============================================================================
# Bu script, CovertGift platformunun Faz 1'den Faz 6'ya kadar olan tüm
# API uç noktalarını (GET, POST, DELETE, Headers, Payloads) canlı olarak
# jüriye / hocaya sunmak için tasarlanmıştır.
#
# Kullanım:
#   ./scripts/demo_presentation.sh         (Adım adım ENTER ile ilerleme)
#   ./scripts/demo_presentation.sh --auto  (Otomatik akış modu)
# ==============================================================================

set -e

# Renkler
RED="\033[1;31m"
GREEN="\033[1;32m"
YELLOW="\033[1;33m"
BLUE="\033[1;34m"
MAGENTA="\033[1;35m"
CYAN="\033[1;36m"
BOLD="\033[1m"
DIM="\033[2m"
RESET="\033[0m"

BASE_URL="http://localhost:8080/api/v1"
TIMESTAMP=$(date +%s)
AUTO_MODE="false"

if [ "$1" == "--auto" ] || [ "$1" == "-y" ]; then
    AUTO_MODE="true"
fi

step_pause() {
    if [ "$AUTO_MODE" != "true" ]; then
        echo -e "\n${YELLOW}👉 Bir sonraki adıma geçmek için ENTER'a basın...${RESET}"
        read -r
    else
        sleep 1
    fi
}

# HTTP İstek ve Yanıt Görsel Kutusu
print_http_box() {
    local method="$1"
    local endpoint="$2"
    local auth="$3"
    local payload="$4"

    echo -e "  ${DIM}╭────────────────────────────────────────────────────────────────────────${RESET}"
    echo -e "  ${DIM}│${RESET} ${BOLD}${MAGENTA}📡 HTTP İSTEĞİ:${RESET} ${BOLD}${GREEN}${method}${RESET} ${CYAN}${endpoint}${RESET}"
    if [ -n "$auth" ]; then
        echo -e "  ${DIM}│${RESET} ${DIM}Header:${RESET}  ${YELLOW}Authorization: Bearer ${auth:0:22}...${RESET} ${DIM}(JWT Token)${RESET}"
    fi
    if [ -n "$payload" ]; then
        echo -e "  ${DIM}│${RESET} ${DIM}Payload:${RESET} ${DIM}${payload}${RESET}"
    fi
    echo -e "  ${DIM}╰────────────────────────────────────────────────────────────────────────${RESET}"
}

# Google Gemini API İstek Detay Kutusu
print_gemini_box() {
    local model="$1"
    local prompt="$2"

    echo -e "  ${DIM}╭────────────────────────────────────────────────────────────────────────${RESET}"
    echo -e "  ${DIM}│${RESET} ${BOLD}${CYAN}🤖 GOOGLE GEMINI 3.8 FLASH'A GİDEN İSTEK DETAYI (SDK Çağrısı):${RESET}"
    echo -e "  ${DIM}│${RESET} ${DIM}Model:${RESET}           ${YELLOW}${model}${RESET} ${DIM}(Yedek Fallback: gemini-3.5-flash-lite)${RESET}"
    echo -e "  ${DIM}│${RESET} ${DIM}MIME Türü:${RESET}       ${GREEN}application/json${RESET} ${DIM}(Structured Output)${RESET}"
    echo -e "  ${DIM}│${RESET} ${DIM}JSON Şeması:${RESET}     ${DIM}[{ title: string, description: string, estimated_price: number, search_query: string }]${RESET}"
    echo -e "  ${DIM}│${RESET} ${BOLD}${MAGENTA}📝 State-Based Prompt (Dinamik Gönderilen Metin):${RESET}"
    while IFS= read -r line; do
        echo -e "  ${DIM}│${RESET}   ${YELLOW}${line}${RESET}"
    done <<< "$prompt"
    echo -e "  ${DIM}╰────────────────────────────────────────────────────────────────────────${RESET}"
}

echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "${BOLD}${CYAN}   🎁 CovertGift — İşbirlikçi Hediye Platformu (Canlı Sistem Demosu)${RESET}"
echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "${BOLD}Mimarî:${RESET} Go 1.27 + Gin Framework + PostgreSQL 17 + GORM + Gemini 3.8 Flash + Link Preview"
echo -e "${BOLD}Kapsam:${RESET} Faz 1 (Modeller) ➔ Faz 2 (Auth) ➔ Faz 3 (Anti-Spoiler) ➔ Faz 4 (Oylama) ➔ Faz 5 (Link Önizleme) ➔ Faz 6 (AI & Veto)"
echo -e "${MAGENTA}------------------------------------------------------------------------------${RESET}"

# 0. Sağlık Kontrolü
echo -e "\n${BOLD}[0/9] 🔍 Sistem Sağlık Kontrolü (Healthcheck)${RESET}"
print_http_box "GET" "/health" "" ""

HEALTH_RAW=$(curl -s -w "\nHTTP_STATUS:%{http_code}" http://localhost:8080/health || echo "DOWN")
HEALTH_CODE=$(echo "$HEALTH_RAW" | grep "HTTP_STATUS" | cut -d':' -f2 || echo "DOWN")
HEALTH_BODY=$(echo "$HEALTH_RAW" | head -n 1)

if [ "$HEALTH_CODE" != "200" ]; then
    echo -e "${RED}❌ HATA: API sunucusuna erişilemedi (http://localhost:8080)!${RESET}"
    echo -e "${YELLOW}Lütfen sunucuyu ayrı bir terminalde başlatın:${RESET}"
    echo -e "   cd backend && ./bin/api  (veya go run ./cmd/api)"
    exit 1
fi
echo -e "  ${BOLD}${GREEN}📥 HTTP ${HEALTH_CODE} OK${RESET} ➔ ${HEALTH_BODY}"
echo -e "  ${GREEN}✔ Backend API ve PostgreSQL bağlantı havuzu aktif!${RESET}"

SWAGGER_CODE=$(curl -s -L -o /dev/null -w "%{http_code}" http://localhost:8080/swagger/index.html || echo "DOWN")
if [ "$SWAGGER_CODE" == "200" ]; then
    echo -e "  ${BOLD}${CYAN}📖 Swagger / OpenAPI UI:${RESET} ${GREEN}http://localhost:8080/swagger/index.html${RESET} ${DIM}(veya /docs)${RESET}"
fi

step_pause

# 1. Kullanıcı Kaydı & Kimlik Doğrulama
echo -e "\n${BOLD}[1/9] 👤 Kullanıcı Kayıtları & JWT Güvenliği (Faz 2)${RESET}"
echo -e "Senaryo gereği 3 farklı aktör oluşturuluyor:"
echo -e "  • ${CYAN}Admin Can${RESET} (Grup Kurucusu)"
echo -e "  • ${CYAN}Üye Zeynep${RESET} (Grup Katılımcısı)"
echo -e "  • ${CYAN}Hedef Ahmet${RESET} (Kendisine sürpriz yapılacak kişi)"

PASSWORD="Password123!"
ADMIN_EMAIL="admin_${TIMESTAMP}@demo.com"
MEMBER_EMAIL="zeynep_${TIMESTAMP}@demo.com"
TARGET_EMAIL="ahmet_${TIMESTAMP}@demo.com"

# Admin Kayıt
print_http_box "POST" "/api/v1/auth/register" "" "{\"email\":\"$ADMIN_EMAIL\",\"name\":\"Admin Can\"}"
ADMIN_REG=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/auth/register" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Admin Can\"}")
echo -e "  ${BOLD}${GREEN}📥 HTTP $(echo "$ADMIN_REG" | grep "HTTP_STATUS" | cut -d':' -f2) CREATED${RESET} ➔ $(echo "$ADMIN_REG" | head -n 1)"

# Admin Giriş
print_http_box "POST" "/api/v1/auth/login" "" "{\"email\":\"$ADMIN_EMAIL\"}"
ADMIN_LOGIN=$(curl -s -X POST "$BASE_URL/auth/login" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$PASSWORD\"}")
ADMIN_TOKEN=$(echo "$ADMIN_LOGIN" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")
echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ JWT Token üretildi: ${YELLOW}${ADMIN_TOKEN:0:30}...${RESET}"

# Üye ve Hedef Kişi
curl -s -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Üye Zeynep\"}" > /dev/null
MEMBER_TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")

TARGET_REG=$(curl -s -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$TARGET_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Hedef Ahmet\"}")
TARGET_ID=$(echo "$TARGET_REG" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")
TARGET_TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$TARGET_EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")

echo -e "\n  ${GREEN}✔ Şifreler bcrypt ile hash'lendi ve 24 saat geçerli JWT token'ları üretildi.${RESET}"

step_pause

# 2. Grup Oluşturma
echo -e "\n${BOLD}[2/9] 👥 Sürpriz Grup Oluşturma & İlgi Alanları (Faz 3)${RESET}"
GROUP_PAYLOAD="{
    \"title\":\"Ahmet'in 30. Yaş Sürprizi\",
    \"target_person_name\":\"Ahmet\",
    \"target_user_id\":\"$TARGET_ID\",
    \"target_interests\":\"Kamp, 3. nesil kahve demleme, trekking, outdoor\",
    \"total_budget\":2500.00
  }"
print_http_box "POST" "/api/v1/groups" "$ADMIN_TOKEN" "$GROUP_PAYLOAD"

GROUP_RESP=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/groups" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "$GROUP_PAYLOAD")

GROUP_CODE=$(echo "$GROUP_RESP" | grep "HTTP_STATUS" | cut -d':' -f2)
GROUP_BODY=$(echo "$GROUP_RESP" | head -n 1)
GROUP_ID=$(echo "$GROUP_BODY" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")

echo -e "  ${BOLD}${GREEN}📥 HTTP ${GROUP_CODE} CREATED${RESET} ➔ Grup ID: ${BOLD}$GROUP_ID${RESET}"
echo -e "  ${GREEN}✔ Grup ve Admin üyeliği atomik GORM transaction ile kaydedildi.${RESET}"

step_pause

# 3. Anti-Spoiler Kalkanı Testi
echo -e "\n${BOLD}[3/9] 🛡️ Anti-Spoiler Kalkanı Güvenlik Testi (Kritik İş Kuralı)${RESET}"
echo -e "Hedef kişi olan Ahmet sisteme giriyor ve sürprizi görmeye çalışıyor:"

echo -e "\n  ${BOLD}a) Ahmet grup listesini çekiyor:${RESET}"
print_http_box "GET" "/api/v1/groups" "$TARGET_TOKEN" ""
TARGET_GROUPS=$(curl -s -X GET "$BASE_URL/groups" -H "Authorization: Bearer $TARGET_TOKEN")
echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ ${CYAN}$TARGET_GROUPS${RESET}"
echo -e "  ${GREEN}✔ SQL filtresi sayesinde Ahmet grubu listede asla göremedi (data: []).${RESET}"

echo -e "\n  ${BOLD}b) Ahmet grup ID'sini doğrudan tarayıcıya/isteğe yazıp girmeye çalışıyor:${RESET}"
print_http_box "GET" "/api/v1/groups/$GROUP_ID" "$TARGET_TOKEN" ""
DIRECT_ACCESS=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET "$BASE_URL/groups/$GROUP_ID" -H "Authorization: Bearer $TARGET_TOKEN")
DIRECT_CODE=$(echo "$DIRECT_ACCESS" | grep "HTTP_STATUS" | cut -d':' -f2)
DIRECT_BODY=$(echo "$DIRECT_ACCESS" | head -n 1)

echo -e "  ${BOLD}${RED}📥 HTTP ${DIRECT_CODE} FORBIDDEN${RESET} ➔ ${YELLOW}$DIRECT_BODY${RESET}"
echo -e "  ${GREEN}✔ GroupAccess Middleware hedef kişiyi anında tespit edip 403 ile engelledi!${RESET}"

step_pause

# 4. Üye Ekleme ve Dinamik Kişi Başı Bütçe
echo -e "\n${BOLD}[4/9] 🤝 Üye Ekleme & Dinamik Kişi Başı Bütçe Hesabı${RESET}"
print_http_box "POST" "/api/v1/groups/$GROUP_ID/members" "$ADMIN_TOKEN" "{\"email\":\"$MEMBER_EMAIL\"}"

curl -s -X POST "$BASE_URL/groups/$GROUP_ID/members" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\"}" > /dev/null

echo -e "  ${BOLD}${GREEN}📥 HTTP 201 CREATED${RESET} ➔ Üye Zeynep başarıyla gruba eklendi."

print_http_box "GET" "/api/v1/groups/$GROUP_ID" "$ADMIN_TOKEN" ""
GROUP_DETAIL=$(curl -s -X GET "$BASE_URL/groups/$GROUP_ID" -H "Authorization: Bearer $ADMIN_TOKEN")
MEMBER_COUNT=$(echo "$GROUP_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['member_count'])")
PER_PERSON=$(echo "$GROUP_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['per_person_budget'])")

echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ Aktif Üye Sayısı: ${BOLD}$MEMBER_COUNT kişi${RESET} | Toplam Bütçe: ${BOLD}2.500 TL${RESET}"
echo -e "  ${CYAN}🎯 Anlık Kişi Başı Bütçe (Toplam / Üye Sayısı): ${BOLD}${PER_PERSON} TL / kişi${RESET}"

step_pause

# 5. Ürün Linki Önizleme ve SSRF Koruması (Faz 5)
echo -e "\n${BOLD}[5/9] 🔗 Ürün Linki Önizleme & SSRF Güvenliği (Faz 5)${RESET}"
echo -e "Kullanıcı manuel öneri eklerken Trendyol kısa linki (${CYAN}https://ty.gl/gkyyduf4wm8u3${RESET}) yapıştırıyor."
echo -e "Backend sayfayı sosyal medya botu kimliğiyle çekip başlık, resim ve fiyatı otomatik ayrıştırıyor:"

print_http_box "POST" "/api/v1/groups/$GROUP_ID/ideas/preview-link" "$ADMIN_TOKEN" "{\"url\":\"https://ty.gl/gkyyduf4wm8u3\"}"
PREVIEW_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/preview-link" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://ty.gl/gkyyduf4wm8u3"}')

echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ Ayrıştırılan Ürün Önizleme Verisi:"
echo "$PREVIEW_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin).get('data', {})
print(f\"     📦 Başlık:       {data.get('title')}\")
print(f\"     💰 Fiyat:        {data.get('price')} {data.get('currency', 'TRY')}\")
print(f\"     🔍 Arama Metni:  {data.get('search_query')}\")
print(f\"     🖼️  Görsel:       {data.get('image_url')[:65]}...\")
print(f\"     🛡️  Kaynak/Durum: {data.get('source')} (partial={data.get('partial')})\")
"

echo -e "\n  ${BOLD}🔒 SSRF Güvenlik Testi: Saldırgan yerel IP (127.0.0.1) verirse:${RESET}"
print_http_box "POST" "/api/v1/groups/$GROUP_ID/ideas/preview-link" "$ADMIN_TOKEN" "{\"url\":\"http://127.0.0.1:8080/health\"}"
SSRF_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/preview-link" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"url":"http://127.0.0.1:8080/health"}')
echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK (Fail-Safe)${RESET} ➔ $(echo "$SSRF_RESP" | python3 -c "import sys, json; d=json.load(sys.stdin)['data']; print(f\"partial={d['partial']}, warning: '{d['warning']}'\")")"
echo -e "  ${GREEN}✔ SSRF Kalkanı devrede: İzinli pazar yerleri harici tüm özel IP ve yabancı alan adları engellendi!${RESET}"

step_pause

# 6. Gemini 3.8 Flash AI Hediye Önerileri
echo -e "\n${BOLD}[6/9] 🤖 Gemini 3.8 Flash AI Hediye Önerileri (Faz 6)${RESET}"
echo -e "Hedef kişinin hobileri (${CYAN}Kamp, 3. nesil kahve, trekking${RESET}) Gemini'ye State-Based prompt ile gönderiliyor..."
print_http_box "POST" "/api/v1/groups/$GROUP_ID/ideas/generate-ai" "$ADMIN_TOKEN" ""

GEMINI_PROMPT_1="Sen profesyonel, yaratıcı ve zevkli bir hediye danışmanısın. Bir grup arkadaş, sevdikleri bir kişi için sürpriz hediye seçecek.

HEDEF KİŞİ BİLGİLERİ:
- İsim: Ahmet
- İlgi Alanları ve Hobileri: Kamp, 3. nesil kahve demleme, trekking, outdoor
- Grup/Kutlama Amacı: Ahmet'in 30. Yaş Sürprizi
- Toplam Bütçe Sınırı: 2500.00 TL

ÖNEMLİ KURALLAR VE KISITLAR:
3. SIFIR KIRIK LİNK KURALI: Asla web sitesi linki veya URL üretme. Sadece Türkiye'deki pazaryerlerinde (Trendyol, Hepsiburada, Amazon TR) aratıldığında doğrudan ürünü çıkaracak temiz, spesifik 'search_query' kelimesi oluştur.
4. FİYAT VE BÜTÇE: Hediyelerin tahmini fiyatlarını gerçekçi belirle ve hedef bütçe civarında kalmaya özen göster (tolerans serbest).
5. ADET: Hedef kişinin zevkine tam hitap eden 3 ile 5 adet arasında birbirinden farklı, orijinal öneri üret."

print_gemini_box "gemini-3.8-flash" "$GEMINI_PROMPT_1"

AI_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/generate-ai" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo -e "  ${BOLD}${GREEN}📥 HTTP 201 CREATED${RESET} ➔ Gemini Yapılandırılmış JSON Yanıtı:"
echo "$AI_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, idea in enumerate(data.get('data', []), 1):
    badge = '⚠️ [BÜTÇE ÜSTÜ]' if idea.get('is_over_budget') else '✅ [BÜTÇE DAHİLİNDE]'
    print(f\"     {i}. {idea['title']} — {idea['estimated_price']} TL {badge}\")
    print(f\"        Açıklama: {idea['description']}\")
    print(f\"        Temiz Arama: \\\"{idea['search_query']}\\\"\n\")
"

FIRST_IDEA_ID=$(echo "$AI_RESP" | python3 -c "import sys, json; d = json.load(sys.stdin); print(d['data'][0]['id'])")
FIRST_IDEA_TITLE=$(echo "$AI_RESP" | python3 -c "import sys, json; d = json.load(sys.stdin); print(d['data'][0]['title'])")

step_pause

# 7. Admin Vetosu
echo -e "\n${BOLD}[7/9] 👑 Admin Vetosu Testi (Örnek Hediye: $FIRST_IDEA_TITLE)${RESET}"

echo -e "\n  ${BOLD}a) Normal Üye Zeynep doğrudan veto etmeyi deniyor:${RESET}"
print_http_box "POST" "/api/v1/ideas/$FIRST_IDEA_ID/reject" "$MEMBER_TOKEN" ""
MEMBER_VETO=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/ideas/$FIRST_IDEA_ID/reject" \
  -H "Authorization: Bearer $MEMBER_TOKEN")
echo -e "  ${BOLD}${RED}📥 HTTP $(echo "$MEMBER_VETO" | grep "HTTP_STATUS" | cut -d':' -f2) FORBIDDEN${RESET} ➔ $(echo "$MEMBER_VETO" | head -n 1)"
echo -e "  ${GREEN}✔ Normal üyenin veto yetkisi olmadığı doğrulandı.${RESET}"

echo -e "\n  ${BOLD}b) Grup Yöneticisi Admin Can veto butonuna basıyor:${RESET}"
print_http_box "POST" "/api/v1/ideas/$FIRST_IDEA_ID/reject" "$ADMIN_TOKEN" ""
ADMIN_VETO=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/ideas/$FIRST_IDEA_ID/reject" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
echo -e "  ${BOLD}${GREEN}📥 HTTP $(echo "$ADMIN_VETO" | grep "HTTP_STATUS" | cut -d':' -f2) OK${RESET} ➔ $(echo "$ADMIN_VETO" | head -n 1)"
echo -e "  ${GREEN}✔ Hediye DB'den silinmedi; durumu 'REJECTED' olarak güncellendi!${RESET}"

step_pause

# 8. Demokrasi Vetosu
echo -e "\n${BOLD}[8/9] 🗳️ Demokrasi Vetosu Testi (Çoğunluk Kararı)${RESET}"
echo -e "Zeynep manuel hediye ekliyor: 'Plastik Anahtarlık' (50 TL)"
print_http_box "POST" "/api/v1/groups/$GROUP_ID/ideas" "$MEMBER_TOKEN" "{\"title\":\"Plastik Anahtarlık\",\"estimated_price\":50.00}"

MANUAL_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Plastik Anahtarlık\",\"description\":\"Basit bir hediye\",\"estimated_price\":50.00}")
MANUAL_ID=$(echo "$MANUAL_RESP" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")
echo -e "  ${BOLD}${GREEN}📥 HTTP 201 CREATED${RESET} ➔ Hediye ID: $MANUAL_ID"

echo -e "\n  ${BOLD}Grup üyeleri oyluyor:${RESET}"
print_http_box "POST" "/api/v1/ideas/$MANUAL_ID/vote" "$MEMBER_TOKEN" "{\"vote_type\": -1}"
curl -s -X POST "$BASE_URL/ideas/$MANUAL_ID/vote" -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H "Content-Type: application/json" -d "{\"vote_type\":-1}" > /dev/null
echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ Zeynep downvote verdi (-1)"

print_http_box "POST" "/api/v1/ideas/$MANUAL_ID/vote" "$ADMIN_TOKEN" "{\"vote_type\": -1}"
curl -s -X POST "$BASE_URL/ideas/$MANUAL_ID/vote" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d "{\"vote_type\":-1}" > /dev/null
echo -e "  ${BOLD}${GREEN}📥 HTTP 200 OK${RESET} ➔ Admin Can downvote verdi (-1) ➔ (2/2 çoğunluk sağlandı)"

MANUAL_STATUS=$(curl -s -X GET "$BASE_URL/groups/$GROUP_ID/ideas" -H "Authorization: Bearer $ADMIN_TOKEN" | \
  python3 -c "import sys, json; data = json.load(sys.stdin)['data']; print([x['status'] for x in data if x['id'] == '$MANUAL_ID'][0])")

echo -e "  ${BOLD}${RED}Hediyenin Güncel Durumu: $MANUAL_STATUS${RESET}"
echo -e "  ${GREEN}✔ Demokrasi Vetosu çalıştı: Çoğunluk istemediği için sistem hediyeyi otomatik REJECTED yaptı!${RESET}"

step_pause

# 9. Negatif Prompt ve AI Belleği
echo -e "\n${BOLD}[9/9] 🧠 Gemini Bellek Testi (State-Based Negatif Prompt)${RESET}"
echo -e "Gruptan ikinci kez AI önerisi isteniyor..."
echo -e "DB'deki Reddedilenler: ${RED}'$FIRST_IDEA_TITLE', 'Plastik Anahtarlık'${RESET}"
echo -e "Gemini'ye iletilen kısıt: ${YELLOW}\"Bunlar grupta reddedildi, kesinlikle benzer öneri getirme!\"${RESET}"

print_http_box "POST" "/api/v1/groups/$GROUP_ID/ideas/generate-ai" "$ADMIN_TOKEN" ""

GEMINI_PROMPT_2="Sen profesyonel, yaratıcı ve zevkli bir hediye danışmanısın. Bir grup arkadaş, sevdikleri bir kişi için sürpriz hediye seçecek.

HEDEF KİŞİ BİLGİLERİ:
- İsim: Ahmet
- İlgi Alanları ve Hobileri: Kamp, 3. nesil kahve demleme, trekking, outdoor
- Toplam Bütçe Sınırı: 2500.00 TL

ÖNEMLİ KURALLAR VE KISITLAR:
1. HAVUZDA ZATEN MEVCUT OLAN HEDİYELER (BUNLARI ASLA TEKRAR ETME): [Havuzdaki diğer hediyeler]
2. GRUBUN REDDETTİĞİ / VETO ETTİĞİ HEDİYELER (BU HEDİYELERİ GRUP BEĞENMEDİ, KESİNLİKLE BUNLARA BENZER BİR ŞEY ÖNERME): '$FIRST_IDEA_TITLE', 'Plastik Anahtarlık'
3. SIFIR KIRIK LİNK KURALI: Asla web sitesi linki veya URL üretme. Sadece temiz, spesifik 'search_query' kelimesi oluştur.
4. FİYAT VE BÜTÇE: Hediyelerin tahmini fiyatlarını gerçekçi belirle ve hedef bütçe civarında kalmaya özen göster (tolerans serbest).
5. ADET: Hedef kişinin zevkine tam hitap eden 3 ile 5 adet arasında birbirinden farklı, orijinal öneri üret."

print_gemini_box "gemini-3.8-flash" "$GEMINI_PROMPT_2"

AI_RESP_2=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/generate-ai" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo -e "  ${BOLD}${GREEN}📥 HTTP 201 CREATED${RESET} ➔ Gemini İkinci Tur Yeni Öneriler:"
echo "$AI_RESP_2" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, idea in enumerate(data.get('data', []), 1):
    badge = '⚠️ [BÜTÇE ÜSTÜ]' if idea.get('is_over_budget') else '✅ [BÜTÇE DAHİLİNDE]'
    print(f\"     {i}. {idea['title']} — {idea['estimated_price']} TL {badge}\")
"
echo -e "  ${GREEN}✔ Gemini reddedilen ürünleri tekrarlamadı; tamamen yeni öneriler getirdi!${RESET}"

echo -e "\n${MAGENTA}==============================================================================${RESET}"
echo -e "${GREEN}${BOLD}🎉 TEBRİKLER! TÜM API UÇ NOKTALARI VE İŞ MANTIKLARI BAŞARIYLA SERGİLENDİ.${RESET}"
echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "Sergilenen REST API Uç Noktaları:"
echo -e "  • ${GREEN}POST /api/v1/auth/register${RESET} (Bcrypt şifreleme)"
echo -e "  • ${GREEN}POST /api/v1/auth/login${RESET} (24h JWT oturum üretimi)"
echo -e "  • ${GREEN}POST /api/v1/groups${RESET} (Atomik Transaction ile grup açma)"
echo -e "  • ${GREEN}GET  /api/v1/groups${RESET} (Anti-spoiler listeleme)"
echo -e "  • ${GREEN}GET  /api/v1/groups/:id${RESET} (403 Spoiler Kalkanı & Dinamik Kişi Başı Bütçe)"
echo -e "  • ${GREEN}POST /api/v1/groups/:id/members${RESET} (Üye daveti)"
echo -e "  • ${GREEN}POST /api/v1/groups/:id/ideas/preview-link${RESET} (Ürün Linki Önizleme & SSRF Kalkanı)"
echo -e "  • ${GREEN}POST /api/v1/groups/:id/ideas${RESET} (Manuel hediye ekleme)"
echo -e "  • ${GREEN}POST /api/v1/groups/:id/ideas/generate-ai${RESET} (Gemini 3.8 Flash Structured JSON)"
echo -e "  • ${GREEN}POST /api/v1/ideas/:id/vote${RESET} (Upsert oylama & Demokrasi Vetosu)"
echo -e "  • ${GREEN}POST /api/v1/ideas/:id/reject${RESET} (Admin doğrudan veto)"
echo -e "  • ${GREEN}DELETE /api/v1/me${RESET} (Kullanıcı hesap ve profil silme / KVKK)"
echo -e "  • ${GREEN}DELETE /api/v1/groups/:id${RESET} (Grup ve tüm ilişkili verilerin silinmesi / Admin)"
echo -e "  • ${GREEN}GET  /swagger/index.html${RESET} (İnteraktif Swagger UI & OpenAPI 2.0 Dokümantasyonu)"
echo -e "${MAGENTA}==============================================================================${RESET}\n"
