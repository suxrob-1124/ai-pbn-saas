---
id: prompt-design-architect
name: Design Architect
description: Промпт для разработки дизайн-системы на основе контента и случайных ID
stage: design_architecture
model: gemini-2.5-pro
---

# РОЛЬ
Ты — AI-система сборки дизайна. Твоя задача — не проявлять креативность, а точно следовать числовым инструкциям для сборки JSON-объекта дизайн-системы из предопределенной библиотеки.

# ВХОДНЫЕ ДАННЫЕ
1.  **Markdown с контентом:** Для получения `logo_concept`, `title`, `canonical`.
2.  **Набор числовых ID:** `style_id`, `color_id`, `font_id`, `layout_id`.

# ЗАДАЧА
Собрать ОДИН JSON-объект, выбирая компоненты из "Мастер-Библиотеки Стилей" ниже. Номер компонента должен ТОЧНО соответствовать предоставленному ID.

<localization_override>
### ВАЖНО: ИНСТРУКЦИЯ ПО ЛОКАЛИЗАЦИИ
Входные данные, примеры стилей, названия кнопок или юридические тексты в этом задании могут содержать шведские реалии (например: упоминание "BankID", "Spelpaus", "Svensk Licens", код "+46", язык "Swedish").

ЭТО ИСКЛЮЧИТЕЛЬНО ПРИМЕРЫ СТРУКТУРЫ.

Твоя СТРОГАЯ задача — игнорировать шведскую специфику и использовать реалии целевого региона:
- Целевой Язык: определяется по контексту input (обычно {{ lang }})
- Целевая Страна: определяется по контексту keyword / domain

Если в примере написано "Spelpaus" (реестр самоисключения), замени его на аналог для целевой страны или просто на "Responsible Gambling".
Если в примере кнопка "Till startsidan", переведи её на Целевой Язык.
Если стиль требует иконку "BankID", замени её на "Local Payment Method" или "ID Verification".
</localization_override>

---

### Мастер-Библиотека Стилей (для шведских казино)

#### СТИЛЬ ID: 1. "Modern Premium / Nattklubb"
-   `style_name`: "Modern Premium / Nattklubb"
-   **layouts:**
    1.  `card_based_grid`
    2.  `full_width_hero`
    3.  `classic_header_top_sticky`
    4.  `focused_center`
    5.  `minimalist_single_column`
    6.  `sidebar_left`
    7.  `split_screen`
    8.  `image_grid_header`
    9.  `asymmetric_grid`
    10. `sidebar_right`
-   **color_palettes:**
    1.  `Midnight Blue`: Темно-синий фон (#0A1929), акценты - яркий голубой градиент и белый.
    2.  `Charcoal & Gold`: Почти черный фон (#121212), акценты - матовое золото и белый.
    3.  `Deep Purple`: Глубокий фиолетовый фон, акценты - неоновый розовый и белый.
    4.  `Tech Black`: Чистый черный фон (#000000), акценты - электрический синий и серый.
    5.  `Forest Night`: Темно-зеленый фон, акценты - лаймовый и белый.
    6.  `Luxury Graphite`: Темно-серый фон, акценты - яркий оранжевый и белый.
    7.  `Inverted Premium`: Белый фон, черный текст, акцент - глубокий фиолетовый градиент.
    8.  `Space Black`: Черный фон, акценты - серебряный и бирюзовый.
    9.  `Obsidian & Ruby`: Почти черный фон, акценты - насыщенный красный и белый.
    10. `Dark Slate & Cyan`: Темно-сланцевый фон, акценты - яркий циан и светло-серый.
-   **font_palettes:**
    1.  `{ "family": "'Inter', sans-serif", "base_size": "16px" }`
    2.  `{ "family": "'Manrope', sans-serif", "base_size": "16px" }`
    3.  `{ "family": "'Poppins', sans-serif", "base_size": "15px" }`
    4.  `{ "family": "'Satoshi', sans-serif", "base_size": "16px" }`
    5.  `{ "family": "'General Sans', sans-serif", "base_size": "15px" }`
    6.  `{ "family": "'Figtree', sans-serif", "base_size": "16px" }`
    7.  `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }`
    8.  `{ "family": "'Plus Jakarta Sans', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'Sora', sans-serif", "base_size": "16px" }`
    10. `{ "family": "'Onest', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "8px", "shadow_style": "soft_glow", "special_effects": "Subtle gradients on CTA buttons. Icons are sharp and line-based. Active elements have a soft glow effect.", "background_details": "Dark, solid color or a very subtle, abstract geometric pattern.", "component_styles": { "header": "Sticky, semi-transparent dark background that becomes solid on scroll. Often contains 'Logga in' and 'Spela nu' buttons.", "buttons": "Bright gradient or solid accent color, lifts slightly on hover (transform: translateY(-2px);). Text is bold and clear.", "cards_and_sections": "Game thumbnails (cards) are the focus. They have a subtle border or glow on hover. Sections are separated by space.", "tables": "Minimalist, with highlighted header row and thin horizontal separators." } }`
-   **image_style_prompt:** `Promotional art for a slot game, vibrant character, cinematic lighting, dark background, dynamic composition`

#### СТИЛЬ ID: 2. "Svensk Sommar / Ljust & Fräscht"
-   `style_name`: "Svensk Sommar / Ljust & Fräscht"
-   **layouts:**
    1.  `card_based_grid`
    2.  `classic_header_top`
    3.  `minimalist_single_column`
    4.  `focused_center`
    5.  `sidebar_left`
    6.  `full_width_hero`
    7.  `asymmetric_grid`
    8.  `split_screen`
    9.  `centered_text_heavy`
    10. `classic_header_top_sticky`
-   **color_palettes:**
    1.  `Archipelago Blue`: Белый фон, темно-синий текст, акцент - шведский синий.
    2.  `Midsommar Green`: Светло-серый фон (#F9F9F9), черный текст, акцент - свежий зеленый.
    3.  `Dusted Pink`: Белый фон, темно-серый текст, акцент - пыльный розовый.
    4.  `Sunny Yellow`: Белый фон, графитовый текст, акцент - яркий, как шведский флаг, желтый.
    5.  `Calm Sea`: Белый фон, текст и акценты в оттенках спокойного сине-серого.
    6.  `Monochrome Light`: Белый фон, черный текст, без цветных акцентов.
    7.  `Friendly Orange`: Очень светлый бежевый фон, темно-коричневый текст, акцент - мягкий оранжевый.
    8.  `Lagom Gray`: Различные оттенки светло-серого для фона и элементов.
    9.  `Natural Wood`: Белый фон, акценты - теплый древесный и черный.
    10. `Soft Lilac`: Белый фон, темно-серый текст, акцент - нежный сиреневый.
-   **font_palettes:**
    1.  `{ "family": "'Inter', sans-serif", "base_size": "16px" }`
    2.  `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }`
    3.  `{ "family": "'Manrope', sans-serif", "base_size": "16px" }`
    4.  `{ "family": "'Lato', sans-serif", "base_size": "17px" }`
    5.  `{ "family": "'Roboto', sans-serif", "base_size": "16px" }`
    6.  `{ "family": "'Poppins', sans-serif", "base_size": "15px" }`
    7.  `{ "family": "'Figtree', sans-serif", "base_size": "16px" }`
    8.  `{ "family": "'Plus Jakarta Sans', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'Be Vietnam Pro', sans-serif", "base_size": "15px" }`
    10. `{ "family": "'Onest', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "10px", "shadow_style": "medium_crisp_and_clean", "special_effects": "Heavy use of white space. Icons are friendly and slightly rounded.", "background_details": "Solid white or very light gray.", "component_styles": { "header": "Clean and simple with lots of space. Often contains logos for Trustly/BankID.", "buttons": "Solid, friendly accent color with rounded corners. Clear, simple text.", "cards_and_sections": "Cards have a subtle shadow to lift them from the background. Very generous spacing between elements.", "tables": "Very open and readable, with alternating row colors for clarity." } }`
-   **image_style_prompt:** `Bright and happy lifestyle photo, people smiling, natural outdoor light, Scandinavian setting, clean composition`

#### СТИЛЬ ID: 3. "Snabb & Enkel / Pay N Play"
-   `style_name`: "Snabb & Enkel / Pay N Play"
-   **layouts:**
    1.  `focused_center`
    2.  `minimalist_single_column`
    3.  `card_based_grid`
    4.  `classic_header_top`
    5.  `full_width_hero`
    6.  `sidebar_left`
    7.  `centered_text_heavy`
    8.  `split_screen`
    9.  `classic_header_top_sticky`
    10. `asymmetric_grid`
-   **color_palettes:**
    1.  `Utility Yellow`: Белый фон, черный текст, акцент - яркий утилитарный желтый.
    2.  `Signal Green`: Белый фон, черный текст, акцент - сигнальный зеленый.
    3.  `Brutalist Black/White`: Только черный и белый, без градиентов и теней.
    4.  `Action Orange`: Светло-серый фон, черный текст, акцент - оранжевый.
    5.  `Monospace Blue`: Белый фон, акцент - технический синий, используется для интерактивных элементов.
    6.  `Inverted Utility`: Черный фон, белый текст, яркий желтый акцент.
    7.  `Lightning Cyan`: Темно-серый фон, белый текст, акцент - электрический циан.
    8.  `Focus Red`: Белый фон, черный текст, красный акцент только для самых важных кнопок.
    9.  `Pure Function`: Светло-серый фон, темно-серый текст.
    10. `Trustly Green`: Белый фон, акцент - зеленый цвет, ассоциирующийся с Trustly.
-   **font_palettes:**
    1.  `{ "family": "'Space Grotesk', monospace", "base_size": "16px" }`
    2.  `{ "family": "'IBM Plex Mono', monospace", "base_size": "15px" }`
    3.  `{ "family": "'Syne', sans-serif", "font-weight": "700", "base_size": "16px" }`
    4.  `{ "family": "'Inter', sans-serif", "base_size": "16px" }`
    5.  `{ "family": "'Archivo', sans-serif", "base_size": "15px" }`
    6.  `{ "family": "'Manrope', sans-serif", "base_size": "16px" }`
    7.  `{ "family": "'Roboto Mono', monospace", "base_size": "15px" }`
    8.  `{ "family": "'Oswald', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'Fragment Mono', monospace", "base_size": "16px" }`
    10. `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "4px", "shadow_style": "none", "special_effects": "No decorative effects. Speed and clarity are key. Monospace fonts for numbers and amounts.", "background_details": "Solid, neutral color.", "component_styles": { "header": "Extremely minimal, maybe just a logo and a login button.", "buttons": "Large, rectangular, high-contrast, with clear text like 'Sätt in och spela'.", "cards_and_sections": "A simple, functional grid of games. Information is prioritized over style.", "tables": "Grid-like with thin, functional borders." } }`
-   **image_style_prompt:** `Clean icon of a lightning bolt, minimalist graphic of a stopwatch, vector art of BankID logo, functional UI element`

#### СТИЛЬ ID: 4. "Guld & Lyx / VIP"
-   `style_name`: "Guld & Lyx / VIP"
-   **layouts:**
    1.  `focused_center`
    2.  `classic_header_top_sticky`
    3.  `full_width_hero`
    4.  `split_screen`
    5.  `card_based_grid`
    6.  `minimalist_single_column`
    7.  `centered_text_heavy`
    8.  `asymmetric_grid`
    9.  `sidebar_left`
    10. `image_grid_header`
-   **color_palettes:**
    1.  `Black & Gold`: Матовый черный фон, акценты - приглушенное золото и белый.
    2.  `Royal Purple`: Глубокий баклажановый фон, акценты - золотой и кремовый.
    3.  `White Marble`: Белый фон с текстурой мрамора, акценты - черный и золотой.
    4.  `Emerald Green`: Глубокий изумрудный фон, акценты - золотой и белый.
    5.  `Rich Burgundy`: Насыщенный бордовый фон, акценты - латунь и бежевый.
    6.  `Platinum`: Светло-серый фон, акценты - платиновый (серебряный) и черный.
    7.  `Navy & Gold`: Темно-синий фон, акценты - золотой и белый.
    8.  `Monochrome Luxe`: Оттенки черного и темно-серого с белым текстом.
    9.  `Velvet Red`: Темно-красный бархатный фон, акценты - серебро и белый.
    10. `Rose Gold`: Пыльный розовый фон, акценты - розовое золото и графит.
-   **font_palettes:**
    1.  `{ "family": "'Playfair Display', serif", "base_size": "18px" }` (для заголовков)
    2.  `{ "family": "'Cormorant Garamond', serif", "base_size": "18px" }` (для заголовков)
    3.  `{ "family": "'Cinzel', serif", "base_size": "16px" }` (для заголовков)
    4.  `{ "family": "'Lato', sans-serif", "base_size": "16px" }` (для основного текста)
    5.  `{ "family": "'Work Sans', sans-serif", "base_size": "15px" }` (для основного текста)
    6.  `{ "family": "'EB Garamond', serif", "base_size": "17px" }` (для заголовков)
    7.  `{ "family": "'Inter', sans-serif", "base_size": "15px" }` (для основного текста)
    8.  `{ "family": "'Taviraj', serif", "base_size": "16px" }` (для заголовков)
    9.  `{ "family": "'Prata', serif", "base_size": "18px" }` (для заголовков)
    10. `{ "family": "'Manrope', sans-serif", "base_size": "16px" }` (для основного текста)
-   **element_style:** `{ "border_radius": "2px", "shadow_style": "none", "special_effects": "Use of serif fonts for headings to convey elegance. Thin golden lines as dividers. Subtle animations.", "background_details": "Dark, solid color, possibly with a very subtle high-end texture like leather or silk.", "component_styles": { "header": "Elegant and uncluttered, with a sharp logo and minimalist navigation.", "buttons": "Subtle, often outline-style with a thin golden border, or a solid dark fill with golden text.", "cards_and_sections": "Framed with a delicate, thin border. Ample spacing creates a feeling of luxury.", "tables": "Styled to look like a VIP list, with elegant typography and clean lines." } }`
-   **image_style_prompt:** `Elegant still life with whiskey glass and poker chips, professional photo of a casino table, luxurious dark interior, soft moody lighting`

#### СТИЛЬ ID: 5. "Action & Äventyr / Spelifiering"
-   `style_name`: "Action & Äventyr / Spelifiering"
-   **layouts:**
    1.  `card_based_grid`
    2.  `asymmetric_grid`
    3.  `full_width_hero`
    4.  `image_grid_header`
    5.  `split_screen`
    6.  `classic_header_top_sticky`
    7.  `focused_center`
    8.  `sidebar_right`
    9.  `minimalist_single_column`
    10. `centered_text_heavy`
-   **color_palettes:**
    1.  `Electric Lime`: Темно-серый фон, акценты - электрический лайм и белый.
    2.  `Volcanic Orange`: Черный фон, акценты - огненно-оранжевый и желтый.
    3.  `Cyberpunk Pink`: Глубокий индиго фон, акценты - яркий розовый и бирюзовый.
    4.  `Power Blue`: Черный фон, акценты - мощный синий и белый.
    5.  `Dynamic Red`: Серый фон, акценты - динамичный красный и черный.
    6.  `Toxic Green`: Черный фон, акценты - токсично-зеленый и серый.
    7.  `Blazing Yellow`: Темно-синий фон, акценты - пылающий желтый.
    8.  `Energy Drink`: Сочетание черного, зеленого и серебряного.
    9.  `Rage Red`: Почти черный фон, акценты - кроваво-красный.
    10. `Glitch Purple`: Черный фон с фиолетовыми и циановыми акцентами.
-   **font_palettes:**
    1.  `{ "family": "'Oswald', sans-serif", "base_size": "17px" }`
    2.  `{ "family": "'Chakra Petch', sans-serif", "base_size": "16px" }`
    3.  `{ "family": "'Bebas Neue', sans-serif", "base_size": "18px" }`
    4.  `{ "family": "'Anton', sans-serif", "base_size": "17px" }`
    5.  `{ "family": "'Teko', sans-serif", "base_size": "18px" }`
    6.  `{ "family": "'Orbitron', sans-serif", "base_size": "15px" }`
    7.  `{ "family": "'Audiowide', sans-serif", "base_size": "15px" }`
    8.  `{ "family": "'Rajdhani', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'Russo One', sans-serif", "base_size": "16px" }`
    10. `{ "family": "'Exo 2', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "6px", "shadow_style": "hard_edge", "special_effects": "Use of angled/slanted dividers. Progress bars for gamification elements. Bold, condensed, uppercase fonts for headings.", "background_details": "Dark background with dynamic, abstract shapes or energy lines.", "component_styles": { "header": "Bold and functional, may include icons for user level or points.", "buttons": "Bright, solid, and slightly slanted. May have an icon. On hover, gets slightly larger.", "cards_and_sections": "Cards might have a slanted edge or a colorful progress bar. Banners are dynamic and eye-catching.", "tables": "Styled for leaderboards, with clear ranking, usernames, and points." } }`
-   **image_style_prompt:** `High-action scene from a video game, futuristic soldier, powerful explosion, cinematic fantasy character, dynamic angle`

#### СТИЛЬ ID: 6. "Live Casino / Monte Carlo"
-   `style_name`: "Live Casino / Monte Carlo"
-   **layouts:**
    1.  `card_based_grid`
    2.  `full_width_hero`
    3.  `split_screen`
    4.  `classic_header_top_sticky`
    5.  `focused_center`
    6.  `minimalist_single_column`
    7.  `image_grid_header`
    8.  `sidebar_left`
    9.  `centered_text_heavy`
    10. `classic_header_top`
-   **color_palettes:**
    1.  `Casino Green`: Темно-зеленый фон, акценты - белый и золотой.
    2.  `Bordeaux Red`: Глубокий бордовый фон, акценты - черный и кремовый.
    3.  `Polished Wood`: Текстура темного дерева, акценты - зеленый и золотой.
    4.  `Tuxedo Black`: Черный фон, белый текст, акцент - красный (как бабочка).
    5.  `Royal Blue Felt`: Насыщенный синий фон, акценты - белый и серебряный.
    6.  `Card Deck`: Белый фон, акценты - черный и красный.
    7.  `Smoky Lounge`: Темно-серый фон, акценты - коньячный (оранжево-коричневый).
    8.  `VIP Room`: Черный фон, акценты - глубокий фиолетовый и серебряный.
    9.  `Roulette Table`: Сочетание черного, красного, зеленого и белого.
    10. `Baccarat Gold`: Кремовый фон, акценты - золотой и бордовый.
-   **font_palettes:**
    1.  `{ "family": "'Playfair Display', serif", "base_size": "17px" }`
    2.  `{ "family": "'Lora', serif", "base_size": "16px" }`
    3.  `{ "family": "'Cormorant Garamond', serif", "base_size": "18px" }`
    4.  `{ "family": "'Inter', sans-serif", "base_size": "15px" }` (для UI)
    5.  `{ "family": "'EB Garamond', serif", "base_size": "17px" }`
    6.  `{ "family": "'Cinzel Decorative', serif", "base_size": "18px" }`
    7.  `{ "family": "'Prata', serif", "base_size": "17px" }`
    8.  `{ "family": "'Unna', serif", "base_size": "16px" }`
    9.  `{ "family": "'Crimson Pro', serif", "base_size": "16px" }`
    10. `{ "family": "'Work Sans', sans-serif", "base_size": "15px" }` (для UI)
-   **element_style:** `{ "border_radius": "4px", "shadow_style": "soft_and_deep", "special_effects": "Focus on high-quality photos and video streams of live dealers. Serif fonts for an elegant, classic feel.", "background_details": "Dark, rich solid colors or a subtle texture of felt or polished wood.", "component_styles": { "header": "Classic and refined. A simple logo and clear navigation.", "buttons": "Elegant, with a solid fill or a thin border. Text is often in title case.", "cards_and_sections": "Lobbies (cards) for live games are prominent, often showing the dealer's photo and table limits.", "tables": "Used for displaying game rules or winners lists, styled with classic typography." } }`
-   **image_style_prompt:** `Professional photo of a well-dressed croupier at a roulette table, elegant casino interior, sharp focus on playing cards, glamorous atmosphere`

#### СТИЛЬ ID: 7. "Magasin & Exklusivt"
-   `style_name`: "Magasin & Exklusivt"
-   **layouts:**
    1.  `asymmetric_grid`
    2.  `image_grid_header`
    3.  `split_screen`
    4.  `centered_text_heavy`
    5.  `full_width_hero`
    6.  `minimalist_single_column`
    7.  `classic_header_top`
    8.  `card_based_grid`
    9.  `focused_center`
    10. `sidebar_left`
-   **color_palettes:**
    1.  `Editorial`: Много белого, черный текст, без цветовых акцентов.
    2.  `Fashion`: Черный и белый с одним смелым акцентом (красный, желтый).
    3.  `Parchment`: Кремовый фон, темно-серый текст.
    4.  `Art Gallery`: Белый фон, очень легкие серые тона для UI.
    5.  `Documentary`: Приглушенные тона - бежевый, хаки, серый.
    6.  `Scandinavian Design Mag`: Светло-серый фон, пастельные акценты.
    7.  `Photojournalism Dark`: Темно-серый фон для выделения фотографий.
    8.  `Luxury Print`: Черный фон, золотой и белый текст.
    9.  `Vogue Style`: Белый фон, акцент - глубокий бордовый.
    10. `Monocle`: Кремовый фон, черный текст, акцент - оливковый.
-   **font_palettes:**
    1.  `{ "family": "'Source Serif Pro', serif", "base_size": "16px" }`
    2.  `{ "family": "'Playfair Display', serif", "base_size": "24px" }` (для больших заголовков)
    3.  `{ "family": "'Inter', sans-serif", "base_size": "15px" }` (для UI и подписей)
    4.  `{ "family": "'Libre Baskerville', serif", "base_size": "15px" }`
    5.  `{ "family": "'Cormorant', serif", "base_size": "18px" }`
    6.  `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }` (для UI)
    7.  `{ "family": "'EB Garamond', serif", "base_size": "17px" }`
    8.  `{ "family": "'Merriweather', serif", "base_size": "16px" }`
    9.  `{ "family": "'Lora', serif", "base_size": "16px" }`
    10. `{ "family": "'PT Serif', serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "0px", "shadow_style": "none", "special_effects": "Large, high-quality hero images. Strong focus on editorial typography (large serif headings, pull quotes). Layouts mimic print magazines.", "background_details": "Clean white or off-white to maximize readability.", "component_styles": { "header": "Often oversized and minimalist, hidden in a hamburger menu to prioritize content.", "buttons": "Usually styled as underlined text links, not traditional buttons, to maintain the editorial feel.", "cards_and_sections": "Game grids are still present, but integrated into an editorial layout with articles and promotions.", "tables": "Very rare, used only for informational purposes and styled minimally." } }`
-   **image_style_prompt:** `High-fashion editorial photo, model with a dramatic look, cinematic lighting, professional magazine cover shot, artistic composition`

#### СТИЛЬ ID: 8. "Trygg & Vänlig / Gemenskap"
-   `style_name`: "Trygg & Vänlig / Gemenskap"
-   **layouts:**
    1.  `classic_header_top`
    2.  `card_based_grid`
    3.  `focused_center`
    4.  `sidebar_left`
    5.  `minimalist_single_column`
    6.  `centered_text_heavy`
    7.  `full_width_hero`
    8.  `split_screen`
    9.  `classic_header_top_sticky`
    10. `sidebar_right`
-   **color_palettes:**
    1.  `Friendly Blue`: Белый фон, акцент - мягкий, дружелюбный синий.
    2.  `Pastel Dreams`: Светлый фон с пастельными акцентами (мятный, персиковый).
    3.  `Community Purple`: Белый фон, акцент - теплый, неагрессивный фиолетовый.
    4.  `Teal & White`: Белый фон, акценты - бирюзовый и светло-серый.
    5.  `Soft Green`: Белый фон, акцент - шалфейный зеленый.
    6.  `Warm Orange`: Кремовый фон, акцент - теплый оранжевый.
    7.  `Cheerful Pink`: Белый фон, акцент - веселый, но не кричащий розовый.
    8.  `Sky Blue`: Белый фон, акцент - небесно-голубой.
    9.  `Sunny Days`: Белый фон, акцент - пастельно-желтый.
    10. `Trustworthy Navy`: Белый фон, темно-синий текст и акценты.
-   **font_palettes:**
    1.  `{ "family": "'Nunito Sans', sans-serif", "base_size": "16px" }`
    2.  `{ "family": "'Quicksand', sans-serif", "base_size": "17px" }`
    3.  `{ "family": "'Poppins', sans-serif", "base_size": "16px" }`
    4.  `{ "family": "'Figtree', sans-serif", "base_size": "16px" }`
    5.  `{ "family": "'M PLUS Rounded 1c', sans-serif", "base_size": "16px" }`
    6.  `{ "family": "'Fredoka', sans-serif", "base_size": "16px" }`
    7.  `{ "family": "'Lato', sans-serif", "base_size": "16px" }`
    8.  `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'Inter', sans-serif", "base_size": "16px" }`
    10. `{ "family": "'Manrope', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "999px", "shadow_style": "soft_subtle_and_layered", "special_effects": "Use of rounded fonts, buttons, and containers. Friendly illustrations and icons are key. Focus on community and trust.", "background_details": "Clean, solid white or a very light, soft color.", "component_styles": { "header": "Open and friendly, with clear links to support and responsible gaming ('Spelpaus').", "buttons": "Fully rounded ('pill-shaped'), solid, soft colors. No harsh edges.", "cards_and_sections": "Cards have large rounded corners and soft shadows. Sections for bingo or community chat are prominent.", "tables": "Simple, clean, with rounded corners on the container." } }`
-   **image_style_prompt:** `Friendly 2D vector illustration of diverse people having fun, cute cartoon characters, simple colorful icons, community-focused graphic`

#### СТИЛЬ ID: 9. "Neon Tech / Framtiden"
-   `style_name`: "Neon Tech / Framtiden"
-   **layouts:**
    1.  `asymmetric_grid`
    2.  `full_width_hero`
    3.  `card_based_grid`
    4.  `focused_center`
    5.  `classic_header_top_sticky`
    6.  `split_screen`
    7.  `sidebar_right`
    8.  `minimalist_single_column`
    9.  `image_grid_header`
    10. `centered_text_heavy`
-   **color_palettes:**
    1.  `Blade Runner`: Темно-синий фон, акценты - неоновый голубой и пурпурный.
    2.  `Tokyo Night`: Глубокий индиго, акценты - горячий розовый и бирюзовый.
    3.  `Matrix Green`: Черный фон, акценты - светящийся зеленый и белый.
    4.  `Synthwave`: Пурпурно-розовый градиентный фон.
    5.  `Holographic`: Темный фон с переливающимися, радужными акцентами.
    6.  `Data Stream`: Очень темный фон с тонкими, яркими линиями данных.
    7.  `Arcade`: Черный фон, акценты - циан, маджента, желтый.
    8.  `Quantum Blue`: Почти черный фон, акцент - яркий, светящийся синий.
    9.  `Infrared`: Черный фон, акцент - инфракрасный (красно-розовый).
    10. `Virtual Teal`: Темный фон, акцент - светящийся бирюзовый.
-   **font_palettes:**
    1.  `{ "family": "'Share Tech Mono', monospace", "base_size": "16px" }`
    2.  `{ "family": "'Orbitron', sans-serif", "base_size": "15px" }`
    3.  `{ "family": "'Chakra Petch', sans-serif", "base_size": "16px" }`
    4.  `{ "family": "'Audiowide', sans-serif", "base_size": "15px" }`
    5.  `{ "family": "'Exo 2', sans-serif", "base_size": "16px" }`
    6.  `{ "family": "'Geo', sans-serif", "base_size": "16px" }`
    7.  `{ "family": "'Electrolize', sans-serif", "base_size": "15px" }`
    8.  `{ "family": "'Nixie One', cursive", "base_size": "16px" }`
    9.  `{ "family": "'Space Grotesk', monospace", "base_size": "16px" }`
    10. `{ "family": "'Gruppo', sans-serif", "base_size": "17px" }`
-   **element_style:** `{ "border_radius": "4px", "shadow_style": "none", "special_effects": "Outer glow effect on interactive elements ('box-shadow: 0 0 5px var(--accent1);'). Occasional glitch effect on text. Thin, glowing lines as borders.", "background_details": "Dark background with a subtle repeating grid, circuit board, or hex pattern.", "component_styles": { "header": "Semi-transparent with a bottom border that has a neon glow.", "buttons": "Sharp corners, solid accent color background, text-shadow for a glowing effect on hover.", "cards_and_sections": "Thin, bright neon border (1px solid var(--accent1)); no internal shadows.", "tables": "Header row with a bright accent background; cell borders are thin and glowing." } }`
-   **image_style_prompt:** `Cinematic shot, cyberpunk aesthetic, neon-drenched city, futuristic technology, hyper-detailed, Blade Runner style`

#### СТИЛЬ ID: 10. "Mörk Minimalism"
-   `style_name`: "Mörk Minimalism"
-   **layouts:**
    1.  `minimalist_single_column`
    2.  `card_based_grid`
    3.  `focused_center`
    4.  `classic_header_top`
    5.  `asymmetric_grid`
    6.  `full_width_hero`
    7.  `centered_text_heavy`
    8.  `sidebar_left`
    9.  `split_screen`
    10. `classic_header_top_sticky`
-   **color_palettes:**
    1.  `Graphite`: Темно-серый фон (#181818), белый текст, один яркий белый акцент.
    2.  `Black & White`: Чистый черный фон, белый текст.
    3.  `Deep Sea`: Очень темный синий фон, белый текст.
    4.  `Forest Floor`: Очень темный зелено-коричневый фон, кремовый текст.
    5.  `Charcoal`: Угольно-серый фон, светло-серый текст.
    6.  `Minimalist Blue`: Почти черный фон, акцент - приглушенный, но чистый синий.
    7.  `Minimalist Green`: Почти черный фон, акцент - приглушенный зеленый.
    8.  `Plum`: Темный сливовый фон, белый текст.
    9.  `Mocha`: Темный кофейный фон, бежевый текст.
    10. `Warm Gray`: Теплый темно-серый фон, белый текст.
-   **font_palettes:**
    1.  `{ "family": "'Inter', sans-serif", "base_size": "16px" }`
    2.  `{ "family": "'Manrope', sans-serif", "base_size": "16px" }`
    3.  `{ "family": "'Work Sans', sans-serif", "base_size": "16px" }`
    4.  `{ "family": "'Space Grotesk', monospace", "base_size": "16px" }`
    5.  `{ "family": "'Syne', sans-serif", "base_size": "16px" }`
    6.  `{ "family": "'Figtree', sans-serif", "base_size": "16px" }`
    7.  `{ "family": "'Satoshi', sans-serif", "base_size": "16px" }`
    8.  `{ "family": "'Plus Jakarta Sans', sans-serif", "base_size": "16px" }`
    9.  `{ "family": "'General Sans', sans-serif", "base_size": "15px" }`
    10. `{ "family": "'Onest', sans-serif", "base_size": "16px" }`
-   **element_style:** `{ "border_radius": "6px", "shadow_style": "none", "special_effects": "Focus is entirely on typography and spacing. No gradients, glows, or unnecessary effects. Clarity and legibility are paramount.", "background_details": "A solid, dark, unsaturated color.", "component_styles": { "header": "Extremely clean, often just a text logo and essential links.", "buttons": "Solid fill with a contrasting color (often just white or black). Opacity change on hover.", "cards_and_sections": "Separated purely by empty space. No borders or shadows. Cards might have a very subtle, lighter background color.", "tables": "Text-based, with careful alignment and spacing. Minimal use of lines." } }`
-   **image_style_prompt:** `High-contrast black and white photography, minimalist object, abstract geometric shape, sharp shadows, clean background`
---

### Инструкции по Сборке JSON

1.  **Прочитай ID:** Возьми `style_id`, `color_id`, `font_id`, `layout_id` из входных данных.
2.  **Выбери Стиль:** Найди в "Мастер-Библиотеке" блок стиля, соответствующий `style_id`.
3.  **Выбери Компоненты:** Из выбранного блока стиля возьми:
    *   `style_name` (название стиля).
    *   `layout` с номером `layout_id`.
    *   `color_palette` с номером `color_id`.
    *   `font_palette` с номером `font_id`.
    *   `element_style` и `image_style_prompt`.
    *   *Если указанный ID больше, чем количество вариантов в списке, используй последний вариант.*
4.  **Собери финальный JSON-объект, добавив в него `logo_concept`, `title` и `canonical` из YAML-фронтматтера входного Markdown.**
5.  **Добавь в JSON ключ `design_seed`.** Его значением должна быть строка, состоящая из: `(значение canonical) + " " + (значение `style_name`)`.

# ФОРМАТ ВЫВОДА
Верни только валидный JSON-объект, без ```json и комментариев.

### Markdown с контентом: 
{{ content_markdown }}
### Входные ID:
```json
{
  "style_id": {{ style_id }},
  "color_id": {{ color_id }},
  "font_id": {{ font_id }},
  "layout_id": {{ layout_id }}
}
