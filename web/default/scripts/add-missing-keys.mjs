import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
    'Failed to start Codex authorization': 'Failed to start Codex authorization',
    'Codex authorization page opened': 'Codex authorization page opened',
    'Failed to complete Codex authorization':
      'Failed to complete Codex authorization',
    'Codex OAuth credential saved': 'Codex OAuth credential saved',
    'Authorize this existing Codex channel independently from your local Codex login.':
      'Authorize this existing Codex channel independently from your local Codex login.',
    'Authorize with Codex': 'Authorize with Codex',
    'Web Search': 'Web Search',
    'Search Provider': 'Search Provider',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo (free, built-in)',
    'Tavily API Key': 'Tavily API Key',
    'Save web search settings': 'Save web search settings',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      'Get a free API key at tavily.com (1,000 requests/month on the free plan).',
  },
  zh: {
    'Failed to start Codex authorization': '启动 Codex 授权失败',
    'Codex authorization page opened': 'Codex 授权页面已打开',
    'Failed to complete Codex authorization': '完成 Codex 授权失败',
    'Codex OAuth credential saved': 'Codex OAuth 凭证已保存',
    'Authorize this existing Codex channel independently from your local Codex login.':
      '为此已有 Codex 渠道单独授权，不受本地 Codex 登录状态影响。',
    'Authorize with Codex': '使用 Codex 授权',
    'Web Search': '联网搜索',
    'Search Provider': '搜索服务商',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo（免费，内置）',
    'Tavily API Key': 'Tavily API Key',
    'Save web search settings': '保存联网搜索设置',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGo 仅支持关键词类查询。如需通用网页搜索请使用 Tavily。所选服务商请求失败时将自动回退到 DuckDuckGo。',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      '可在 tavily.com 免费获取 API Key（免费套餐每月 1,000 次请求）。',
  },
  fr: {
    'Failed to start Codex authorization': 'Échec du démarrage de l’autorisation Codex',
    'Codex authorization page opened': 'Page d’autorisation Codex ouverte',
    'Failed to complete Codex authorization': 'Échec de la finalisation de l’autorisation Codex',
    'Codex OAuth credential saved': 'Identifiants OAuth Codex enregistrés',
    'Authorize this existing Codex channel independently from your local Codex login.':
      'Autorisez ce canal Codex existant indépendamment de votre connexion Codex locale.',
    'Authorize with Codex': 'Autoriser avec Codex',
    'Web Search': 'Recherche Web',
    'Search Provider': 'Fournisseur de recherche',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo (gratuit, intégré)',
    'Tavily API Key': 'Clé API Tavily',
    'Save web search settings': 'Enregistrer les paramètres de recherche Web',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGo ne répond qu’aux requêtes par mots-clés. Pour une recherche Web générale, utilisez Tavily. En cas d’échec du fournisseur sélectionné, DuckDuckGo sert de solution de repli.',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      'Obtenez une clé API gratuite sur tavily.com (1 000 requêtes/mois avec l’offre gratuite).',
  },
  ja: {
    'Failed to start Codex authorization': 'Codexの認証を開始できませんでした',
    'Codex authorization page opened': 'Codexの認証ページを開きました',
    'Failed to complete Codex authorization': 'Codexの認証を完了できませんでした',
    'Codex OAuth credential saved': 'Codex OAuth認証情報を保存しました',
    'Authorize this existing Codex channel independently from your local Codex login.':
      'この既存のCodexチャネルを、ローカルのCodexログインとは別に認証します。',
    'Authorize with Codex': 'Codexで認証',
    'Web Search': 'ウェブ検索',
    'Search Provider': '検索プロバイダー',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo（無料・組み込み）',
    'Tavily API Key': 'Tavily APIキー',
    'Save web search settings': 'ウェブ検索設定を保存',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGoはキーワード形式のクエリにのみ対応します。一般的なウェブ検索にはTavilyを使用してください。選択したプロバイダーが失敗した場合、DuckDuckGoがフォールバックとして使用されます。',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      'tavily.comで無料のAPIキーを取得できます（無料プランは月1,000リクエスト）。',
  },
  ru: {
    'Failed to start Codex authorization': 'Не удалось начать авторизацию Codex',
    'Codex authorization page opened': 'Страница авторизации Codex открыта',
    'Failed to complete Codex authorization': 'Не удалось завершить авторизацию Codex',
    'Codex OAuth credential saved': 'Учетные данные OAuth Codex сохранены',
    'Authorize this existing Codex channel independently from your local Codex login.':
      'Авторизуйте этот существующий канал Codex независимо от локального входа в Codex.',
    'Authorize with Codex': 'Авторизовать через Codex',
    'Web Search': 'Веб-поиск',
    'Search Provider': 'Поисковый провайдер',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo (бесплатно, встроен)',
    'Tavily API Key': 'API-ключ Tavily',
    'Save web search settings': 'Сохранить настройки веб-поиска',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGo отвечает только на запросы по ключевым словам. Для общего веб-поиска используйте Tavily. Если выбранный провайдер недоступен, используется DuckDuckGo как запасной вариант.',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      'Получите бесплатный API-ключ на tavily.com (1000 запросов в месяц на бесплатном тарифе).',
  },
  vi: {
    'Failed to start Codex authorization': 'Không thể bắt đầu ủy quyền Codex',
    'Codex authorization page opened': 'Đã mở trang ủy quyền Codex',
    'Failed to complete Codex authorization': 'Không thể hoàn tất ủy quyền Codex',
    'Codex OAuth credential saved': 'Đã lưu thông tin xác thực OAuth Codex',
    'Authorize this existing Codex channel independently from your local Codex login.':
      'Ủy quyền riêng cho kênh Codex hiện có này, không phụ thuộc lần đăng nhập Codex cục bộ.',
    'Authorize with Codex': 'Ủy quyền với Codex',
    'Web Search': 'Tìm kiếm Web',
    'Search Provider': 'Nhà cung cấp tìm kiếm',
    'DuckDuckGo (free, built-in)': 'DuckDuckGo (miễn phí, tích hợp sẵn)',
    'Tavily API Key': 'Khóa API Tavily',
    'Save web search settings': 'Lưu cài đặt tìm kiếm Web',
    'DuckDuckGo only answers keyword-style queries. For general web search, use Tavily. If the selected provider fails, DuckDuckGo is used as fallback.':
      'DuckDuckGo chỉ trả lời các truy vấn dạng từ khóa. Để tìm kiếm web tổng quát, hãy dùng Tavily. Nếu nhà cung cấp đã chọn gặp lỗi, DuckDuckGo sẽ được dùng làm phương án dự phòng.',
    'Get a free API key at tavily.com (1,000 requests/month on the free plan).':
      'Nhận khóa API miễn phí tại tavily.com (1.000 yêu cầu/tháng với gói miễn phí).',
  },
}

async function main() {
  for (const [locale, keys] of Object.entries(newKeys)) {
    const filePath = path.join(LOCALES_DIR, `${locale}.json`)
    const raw = await fs.readFile(filePath, 'utf8')
    const data = JSON.parse(raw)
    const translation = data.translation ?? {}

    let added = 0
    for (const [key, value] of Object.entries(keys)) {
      if (!(key in translation)) {
        translation[key] = value
        added++
      }
    }

    const sorted = Object.fromEntries(
      Object.entries(translation).sort(([a], [b]) => a.localeCompare(b))
    )
    data.translation = sorted

    await fs.writeFile(filePath, stableStringify(data))
    console.log(`${locale}: added ${added} key(s)`)
  }
}

await main()
