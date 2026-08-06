import fs from 'node:fs/promises'
import path from 'node:path'

const LOCALES_DIR = path.resolve('src/i18n/locales')

function stableStringify(obj) {
  return JSON.stringify(obj, null, 2) + '\n'
}

const newKeys = {
  en: {
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
