// Grup açıklaması kuralları (backend: services.MaxGroupDescription). Sunucu metni kırpar ve
// karakterleri (rune) sayar; istemci de kırpılmış metnin kod noktası sayısını kullanır.
export const MAX_GROUP_DESCRIPTION = 1000

export const descriptionLength = (text: string) => [...text.trim()].length
