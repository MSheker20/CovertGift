# Güvenlik / Security

## Güvenlik açığı bildirimi

Bir güvenlik açığı bulduysanız lütfen **herkese açık bir issue açmayın**. Bunun yerine GitHub'ın
özel bildirim özelliğini kullanın: depo sayfasında **Security → Report a vulnerability**.

Özellikle şu alanlardaki bulgular önemlidir:

- **Anti-Spoiler:** Hediye alınan kişinin kendi grubunu, havuzunu, fikirlerini ya da davet linkini görebilmesi.
- **Yetkilendirme:** Bir kullanıcının üyesi olmadığı bir grubun ya da havuzun verisine erişebilmesi.
- **Link önizleme (`pkg/linkpreview`):** SSRF, özel IP'lere erişim ya da boyut ve yönlendirme sınırlarının aşılması.
- **Kimlik doğrulama:** JWT, parola ya da davet token'ıyla ilgili sorunlar.

Bildiriminize mümkünse tekrar üretme adımlarını ekleyin. Kişisel bir proje olduğu için yanıt süresi
garanti edilmez, ancak her bildirim incelenir.

## Reporting a vulnerability

Please **do not open a public issue**. Use GitHub's private reporting instead:
**Security → Report a vulnerability** on the repository page.
