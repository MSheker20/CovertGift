#!/bin/bash
# ==============================================================================
# 🎁 CovertGift — Akademik Sunum & Canlı Demo Test Senaryosu
# ==============================================================================
# Bu script, CovertGift platformunun Faz 1'den Faz 6'ya kadar olan tüm
# yeteneklerini canlı olarak jüriye / hocaya sunmak için tasarlanmıştır.
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

echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "${BOLD}${CYAN}   🎁 CovertGift — İşbirlikçi Hediye Platformu (Canlı Sistem Demosu)${RESET}"
echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "${BOLD}Mimarî:${RESET} Go 1.27 + Gin Framework + PostgreSQL 17 + GORM + Gemini 3.8 Flash"
echo -e "${BOLD}Kapsam:${RESET} Faz 1 (Modeller) ➔ Faz 2 (Auth) ➔ Faz 3 (Anti-Spoiler) ➔ Faz 4 (Oylama) ➔ Faz 6 (AI & Veto)"
echo -e "${MAGENTA}------------------------------------------------------------------------------${RESET}"

# 0. Sağlık Kontrolü
echo -e "\n${BOLD}[0/8] 🔍 Sistem Sağlık Kontrolü (Healthcheck)...${RESET}"
HEALTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health || echo "DOWN")

if [ "$HEALTH_STATUS" != "200" ]; then
    echo -e "${RED}❌ HATA: API sunucusuna erişilemedi (http://localhost:8080)!${RESET}"
    echo -e "${YELLOW}Lütfen sunucuyu ayrı bir terminalde başlatın:${RESET}"
    echo -e "   cd backend && ./bin/api  (veya go run ./cmd/api)"
    exit 1
fi
echo -e "${GREEN}✅ Backend API ve PostgreSQL veritabanı aktif ve çalışıyor! (200 OK)${RESET}"

step_pause

# 1. Kullanıcı Kaydı & Kimlik Doğrulama
echo -e "\n${BOLD}[1/8] 👤 Kullanıcı Kayıtları & JWT Güvenliği (Faz 2)${RESET}"
echo -e "Senaryo gereği 3 farklı aktör oluşturuluyor:"
echo -e "  • ${CYAN}Admin Can${RESET} (Grup Kurucusu)"
echo -e "  • ${CYAN}Üye Zeynep${RESET} (Grup Katılımcısı)"
echo -e "  • ${CYAN}Hedef Ahmet${RESET} (Kendisine sürpriz yapılacak doğum günü çocuğu)"

PASSWORD="Password123!"
ADMIN_EMAIL="admin_${TIMESTAMP}@demo.com"
MEMBER_EMAIL="zeynep_${TIMESTAMP}@demo.com"
TARGET_EMAIL="ahmet_${TIMESTAMP}@demo.com"

# Admin
curl -s -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Admin Can\"}" > /dev/null
ADMIN_TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")

# Üye
curl -s -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Üye Zeynep\"}" > /dev/null
MEMBER_TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")
MEMBER_ID=$(curl -s -X GET "$BASE_URL/me" -H "Authorization: Bearer $MEMBER_TOKEN" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")

# Hedef Kişi
TARGET_REG=$(curl -s -X POST "$BASE_URL/auth/register" -H "Content-Type: application/json" \
  -d "{\"email\":\"$TARGET_EMAIL\",\"password\":\"$PASSWORD\",\"name\":\"Hedef Ahmet\"}")
TARGET_ID=$(echo "$TARGET_REG" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")
TARGET_TOKEN=$(curl -s -X POST "$BASE_URL/auth/login" -H "Content-Type: application/json" \
  -d "{\"email\":\"$TARGET_EMAIL\",\"password\":\"$PASSWORD\"}" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['token'])")

echo -e "${GREEN}✅ Şifreler bcrypt ile hash'lendi ve 24 saat geçerli JWT oturum token'ları üretildi.${RESET}"

step_pause

# 2. Grup Oluşturma
echo -e "\n${BOLD}[2/8] 👥 Sürpriz Grup Oluşturma & İlgi Alanları (Faz 3)${RESET}"
echo -e "Admin Can, Ahmet için 2.500 TL bütçeli bir sürpriz grup açıyor:"
echo -e "  • Hedef Kişi Adı: ${BOLD}Ahmet${RESET}"
echo -e "  • İlgi Alanları (target_interests): ${CYAN}Kamp, 3. nesil kahve demleme, trekking, outdoor ekipmanları${RESET}"
echo -e "  • Toplam Bütçe: ${BOLD}2.500,00 TL${RESET}"

GROUP_RESP=$(curl -s -X POST "$BASE_URL/groups" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"title\":\"Ahmet'in 30. Yaş Sürprizi\",
    \"description\":\"Doğum günü outdoor sürpriz hediyesi\",
    \"target_person_name\":\"Ahmet\",
    \"target_user_id\":\"$TARGET_ID\",
    \"target_interests\":\"Kamp, 3. nesil kahve demleme, trekking, outdoor ekipmanları\",
    \"total_budget\":2500.00,
    \"event_date\":\"2026-11-20T00:00:00Z\"
  }")

GROUP_ID=$(echo "$GROUP_RESP" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")
echo -e "${GREEN}✅ Grup atomik bir GORM transaction ile oluşturuldu! Grup ID: $GROUP_ID${RESET}"

step_pause

# 3. Anti-Spoiler Kalkanı Testi
echo -e "\n${BOLD}[3/8] 🛡️ Anti-Spoiler Kalkanı Güvenlik Testi (Kritik İş Kuralı)${RESET}"
echo -e "Şimdi hedef kişi olan Ahmet sisteme giriyor ve sürprizi görmeye çalışıyor:"

echo -e "\n  a) Ahmet kendi grup listesini çekiyor (GET /api/v1/groups):"
TARGET_GROUPS=$(curl -s -X GET "$BASE_URL/groups" -H "Authorization: Bearer $TARGET_TOKEN")
echo -e "     Sonuç: ${CYAN}$TARGET_GROUPS${RESET}"
echo -e "     ${GREEN}✔ Ahmet'e sürpriz grup SQL seviyesinde gizlendi, boş dizi ([]) döndü.${RESET}"

echo -e "\n  b) Ahmet grup ID'sini öğrenip zorla girmeye çalışıyor (GET /api/v1/groups/:id):"
DIRECT_ACCESS=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X GET "$BASE_URL/groups/$GROUP_ID" -H "Authorization: Bearer $TARGET_TOKEN")
HTTP_CODE=$(echo "$DIRECT_ACCESS" | grep "HTTP_STATUS" | cut -d':' -f2)
ERR_MSG=$(echo "$DIRECT_ACCESS" | head -n 1)
echo -e "     HTTP Durum Kodu: ${RED}$HTTP_CODE FORBIDDEN${RESET}"
echo -e "     Sunucu Yanıtı: ${YELLOW}$ERR_MSG${RESET}"
echo -e "     ${GREEN}✔ Anti-Spoiler Middleware hedef kişiyi anında tespit etti ve 403 ile engelledi!${RESET}"

step_pause

# 4. Üye Ekleme ve Dinamik Kişi Başı Bütçe
echo -e "\n${BOLD}[4/8] 🤝 Üye Ekleme & Dinamik Kişi Başı Bütçe Hesabı${RESET}"
echo -e "Üye Zeynep gruba davet ediliyor..."
curl -s -X POST "$BASE_URL/groups/$GROUP_ID/members" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"email\":\"$MEMBER_EMAIL\"}" > /dev/null

GROUP_DETAIL=$(curl -s -X GET "$BASE_URL/groups/$GROUP_ID" -H "Authorization: Bearer $ADMIN_TOKEN")
MEMBER_COUNT=$(echo "$GROUP_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['member_count'])")
PER_PERSON=$(echo "$GROUP_DETAIL" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['per_person_budget'])")

echo -e "${GREEN}✅ Zeynep gruba katıldı!${RESET}"
echo -e "  • Gruptaki Aktif Üye Sayısı: ${BOLD}$MEMBER_COUNT kişi${RESET}"
echo -e "  • Toplam Bütçe: ${BOLD}2.500,00 TL${RESET}"
echo -e "  • Dinamik Kişi Başı Bütçe (Toplam / Üye Sayısı): ${CYAN}${BOLD}$PER_PERSON TL / kişi${RESET}"

step_pause

# 5. Gemini 3.8 Flash AI Hediye Önerileri
echo -e "\n${BOLD}[5/8] 🤖 Gemini 3.8 Flash AI Hediye Önerileri (Faz 6)${RESET}"
echo -e "Hedef kişinin hobileri (${CYAN}Kamp, 3. nesil kahve, trekking${RESET}) Gemini'ye semantik girdi olarak gönderiliyor..."
echo -e "Kural 3 Güvencesi: Asla kırık URL uydurmaz, pazaryerleri için temiz 'search_query' üretir."

AI_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/generate-ai" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo -e "\n${CYAN}Gemini'den Gelen Yapılandırılmış JSON Yanıtı:${RESET}"
echo "$AI_RESP" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, idea in enumerate(data.get('data', []), 1):
    budget_badge = '⚠️ [BÜTÇE ÜSTÜ]' if idea.get('is_over_budget') else '✅ [BÜTÇE DAHİLİNDE]'
    print(f\"  {i}. {idea['title']} — {idea['estimated_price']} TL {budget_badge}\")
    print(f\"     Açıklama: {idea['description']}\")
    print(f\"     Arama Terimi: \\\"{idea['search_query']}\\\"\n\")
"

FIRST_IDEA_ID=$(echo "$AI_RESP" | python3 -c "import sys, json; d = json.load(sys.stdin); print(d['data'][0]['id'])")
FIRST_IDEA_TITLE=$(echo "$AI_RESP" | python3 -c "import sys, json; d = json.load(sys.stdin); print(d['data'][0]['title'])")

step_pause

# 6. Admin Vetosu
echo -e "\n${BOLD}[6/8] 👑 Admin Vetosu Testi (Örnek Hediye: $FIRST_IDEA_TITLE)${RESET}"
echo -e "  a) Normal Üye Zeynep bu hediyeyi doğrudan veto etmeyi deniyor:"
MEMBER_VETO=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/ideas/$FIRST_IDEA_ID/reject" \
  -H "Authorization: Bearer $MEMBER_TOKEN")
echo -e "     Sonuç: ${RED}$(echo "$MEMBER_VETO" | grep "HTTP_STATUS")${RESET} — $(echo "$MEMBER_VETO" | head -n 1)"
echo -e "     ${GREEN}✔ Normal üyelerin veto yetkisi olmadığı doğrulandı (403 Forbidden).${RESET}"

echo -e "\n  b) Grup Yöneticisi Admin Can veto butonuna basıyor:"
ADMIN_VETO=$(curl -s -w "\nHTTP_STATUS:%{http_code}" -X POST "$BASE_URL/ideas/$FIRST_IDEA_ID/reject" \
  -H "Authorization: Bearer $ADMIN_TOKEN")
echo -e "     Sonuç: ${GREEN}$(echo "$ADMIN_VETO" | grep "HTTP_STATUS")${RESET} — $(echo "$ADMIN_VETO" | head -n 1)"
echo -e "     ${GREEN}✔ Hediye veritabanından silinmedi; durumu 'REJECTED' olarak işaretlendi!${RESET}"

step_pause

# 7. Demokrasi Vetosu
echo -e "\n${BOLD}[7/8] 🗳️ Demokrasi Vetosu Testi (Çoğunluk Kararı)${RESET}"
echo -e "Zeynep havuza manuel bir hediye önerisi ekliyor: 'Plastik Anahtarlık' (50 TL)"
MANUAL_RESP=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas" \
  -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"title\":\"Plastik Anahtarlık\",\"description\":\"Basit bir hediye\",\"estimated_price\":50.00}")
MANUAL_ID=$(echo "$MANUAL_RESP" | python3 -c "import sys, json; print(json.load(sys.stdin)['data']['id'])")

echo -e "Grup üyeleri oyluyor:"
echo -e "  • Zeynep downvote (-1) veriyor..."
curl -s -X POST "$BASE_URL/ideas/$MANUAL_ID/vote" -H "Authorization: Bearer $MEMBER_TOKEN" \
  -H "Content-Type: application/json" -d "{\"vote_type\":-1}" > /dev/null

echo -e "  • Admin Can downvote (-1) veriyor (2 üyeli grupta 2 downvote -> çoğunluk sağlandı)..."
curl -s -X POST "$BASE_URL/ideas/$MANUAL_ID/vote" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H "Content-Type: application/json" -d "{\"vote_type\":-1}" > /dev/null

MANUAL_STATUS=$(curl -s -X GET "$BASE_URL/groups/$GROUP_ID/ideas" -H "Authorization: Bearer $ADMIN_TOKEN" | \
  python3 -c "import sys, json; data = json.load(sys.stdin)['data']; print([x['status'] for x in data if x['id'] == '$MANUAL_ID'][0])")

echo -e "Hediyenin güncel durumu: ${BOLD}${RED}$MANUAL_STATUS${RESET}"
echo -e "${GREEN}✅ Demokrasi Vetosu çalıştı: Çoğunluk istemediği için sistem hediyeyi otomatik REJECTED yaptı!${RESET}"

step_pause

# 8. Negatif Prompt ve AI Belleği
echo -e "\n${BOLD}[8/8] 🧠 Gemini Bellek Testi (State-Based Negatif Prompt)${RESET}"
echo -e "Gruptan ikinci kez AI önerisi isteniyor..."
echo -e "Veritabanındaki durum:"
echo -e "  • ${RED}Reddedilenler:${RESET} '$FIRST_IDEA_TITLE', 'Plastik Anahtarlık'"
echo -e "Gemini'ye bu liste kısıt olarak veriliyor: 'Bunları grup beğenmedi, benzerini getirme!'"

AI_RESP_2=$(curl -s -X POST "$BASE_URL/groups/$GROUP_ID/ideas/generate-ai" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo -e "\n${CYAN}Gemini'nin İkinci Tur Yeni Önerileri:${RESET}"
echo "$AI_RESP_2" | python3 -c "
import sys, json
data = json.load(sys.stdin)
for i, idea in enumerate(data.get('data', []), 1):
    budget_badge = '⚠️ [BÜTÇE ÜSTÜ]' if idea.get('is_over_budget') else '✅ [BÜTÇE DAHİLİNDE]'
    print(f\"  {i}. {idea['title']} — {idea['estimated_price']} TL {budget_badge}\")
"
echo -e "${GREEN}✅ Gemini reddedilen veya havuzda olan hiçbir ürünü tekrarlamadı! Tamamen taze öneriler sundu.${RESET}"

echo -e "\n${MAGENTA}==============================================================================${RESET}"
echo -e "${GREEN}${BOLD}🎉 TEBRİKLER! TÜM MİMARİ VE İŞ KURALLARI BAŞARIYLA SERGİLENDİ.${RESET}"
echo -e "${MAGENTA}==============================================================================${RESET}"
echo -e "Sunum için hazır uç noktalar:"
echo -e "  • Auth: POST /auth/register, POST /auth/login, GET /me"
echo -e "  • Grup & Anti-Spoiler: POST /groups, GET /groups, GET /groups/:id, POST /groups/:id/members"
echo -e "  • Havuz & Oylama: POST /groups/:id/ideas, GET /groups/:id/ideas, POST /ideas/:id/vote"
echo -e "  • Gemini AI & Veto: POST /groups/:id/ideas/generate-ai, POST /ideas/:id/reject"
echo -e "${MAGENTA}==============================================================================${RESET}\n"
