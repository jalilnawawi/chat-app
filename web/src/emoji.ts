/**
 * Daftar emoji untuk pemilih di kolom tulis dan di tombol reaksi.
 *
 * Ditulis tangan, bukan diambil dari pustaka. Pustaka pemilih emoji membawa
 * ribuan entri beserta nama dan kata kuncinya dalam belasan bahasa — ratusan
 * kilobyte untuk sesuatu yang dipakai orang dengan cara menggulir lalu
 * menunjuk. Yang ada di sini adalah yang benar-benar dipakai dalam percakapan
 * sehari-hari, dikelompokkan seperti papan ketik ponsel supaya letaknya
 * terasa akrab.
 *
 * Setiap entri lolos pemeriksaan reaksi di server (server/internal/api/
 * emoji.go): satu grafem, tanpa bendera subdivisi, tanpa rangkaian yang baru
 * muncul tahun lalu. Pemilih yang sama dipakai untuk reaksi, dan emoji yang
 * bisa dipilih tapi ditolak saat ditekan adalah tombol yang berbohong.
 *
 * Emoji yang TIDAK ada di sini tetap bisa diketik lewat papan ketik sistem —
 * daftar ini jalan pintas, bukan batas.
 */
export type EmojiGroup = { id: string; label: string; icon: string; items: string[] };

const split = (s: string) => s.trim().split(/\s+/);

export const EMOJI_GROUPS: EmojiGroup[] = [
  {
    id: 'wajah',
    label: 'Wajah',
    icon: '😀',
    items: split(`
      😀 😃 😄 😁 😆 😅 🤣 😂 🙂 🙃 😉 😊 😇 🥰 😍 🤩 😘 😗 😚 😙 😋 😛 😜 🤪
      😝 🤑 🤗 🤭 🤫 🤔 🤐 🤨 😐 😑 😶 😏 😒 🙄 😬 😌 😔 😪 🤤 😴 😷 🤒 🤕 🤢
      🤮 🤧 🥵 🥶 🥴 😵 🤯 🤠 🥳 😎 🤓 🧐 😕 😟 🙁 😮 😯 😲 😳 🥺 😦 😧 😨 😰
      😥 😢 😭 😱 😖 😣 😞 😓 😩 😫 🥱 😤 😡 😠 🤬 😈 👿 💀 💩 🤡 👻 👽 🤖
      😺 😸 😹 😻 😼 😽 🙀 😿 😾 🙈 🙉 🙊
    `),
  },
  {
    id: 'tangan',
    label: 'Tangan & orang',
    icon: '👍',
    items: split(`
      👍 👎 👌 🤌 🤏 ✌️ 🤞 🤟 🤘 🤙 👈 👉 👆 👇 ☝️ ✋ 🤚 🖐️ 🖖 👋 👏 🙌 👐 🤲
      🤝 🙏 ✍️ 💪 🦾 🫶 👀 👁️ 👄 🧠 🫀 👶 🧒 👦 👧 🧑 👨 👩 🧓 👴 👵 🙋 🙆 🙅
      🤷 🤦 🙇 💁 🧏 🙎 🙍 💃 🕺 🚶 🏃 🧎 🧍 👫 👭 👬 💏 💑 👪
    `),
  },
  {
    id: 'hati',
    label: 'Hati & simbol',
    icon: '❤️',
    items: split(`
      ❤️ 🧡 💛 💚 💙 💜 🤎 🖤 🤍 💔 ❤️‍🔥 ❣️ 💕 💞 💓 💗 💖 💘 💝 💟 💯 💢 💥 💫
      💦 💨 🕳️ 💬 💭 💤 ✨ ⭐ 🌟 ⚡ 🔥 ✅ ☑️ ✔️ ❌ ❎ ➕ ➖ ❓ ❗ ‼️ ⁉️ ⚠️ 🚫
      ⛔ 🔔 🔕 🎵 🎶 ♻️ 🆗 🆒 🆕 🆓 🔝 🔜 🔴 🟠 🟡 🟢 🔵 🟣 ⚫ ⚪
    `),
  },
  {
    id: 'alam',
    label: 'Hewan & alam',
    icon: '🌿',
    items: split(`
      🐶 🐱 🐭 🐹 🐰 🦊 🐻 🐼 🐨 🐯 🦁 🐮 🐷 🐸 🐵 🐔 🐧 🐦 🐤 🦆 🦅 🦉 🦇 🐺
      🐴 🦄 🐝 🐛 🦋 🐌 🐞 🐜 🐢 🐍 🦎 🐙 🦑 🦐 🦀 🐠 🐟 🐬 🐳 🦈 🐊 🐘 🦒 🐪
      🐄 🐐 🐑 🐈 🐓 🦜 🌵 🌲 🌳 🌴 🌱 🌿 🍀 🍁 🍂 🌷 🌹 🌺 🌸 🌼 🌻 🌞 🌝 🌚
      🌙 🌠 🌈 ☀️ ⛅ ☁️ 🌧️ ⛈️ 🌩️ ❄️ ☃️ 🌊 🌋 🌍
    `),
  },
  {
    id: 'makanan',
    label: 'Makanan & minuman',
    icon: '🍜',
    items: split(`
      🍏 🍎 🍐 🍊 🍋 🍌 🍉 🍇 🍓 🍈 🍒 🍑 🥭 🍍 🥥 🥝 🍅 🥑 🍆 🌶️ 🌽 🥕 🥔 🧄
      🧅 🥜 🍞 🥐 🧀 🥚 🍳 🥞 🧇 🥓 🍗 🍖 🌭 🍔 🍟 🍕 🥪 🌮 🌯 🥗 🍝 🍜 🍲 🍛
      🍣 🍱 🥟 🍤 🍙 🍚 🍘 🍢 🍡 🍧 🍨 🍦 🥧 🧁 🍰 🎂 🍮 🍭 🍬 🍫 🍿 🍩 🍪 🥛
      ☕ 🍵 🧃 🥤 🧋 🍶 🍺 🍻 🥂 🍷 🧊
    `),
  },
  {
    id: 'kegiatan',
    label: 'Kegiatan & benda',
    icon: '⚽',
    items: split(`
      ⚽ 🏀 🏈 ⚾ 🎾 🏐 🏓 🏸 🥊 🏆 🥇 🥈 🥉 🏅 🎮 🎲 🧩 🎯 🎳 🎨 🎬 🎤 🎧 🎸
      🎹 🥁 🎉 🎊 🎈 🎁 🎀 🎄 🎆 🎇 🧨 🕌 🕋 🏠 🏢 🏫 🏥 🚗 🚕 🚌 🚑 🚓 🏍️ 🚲
      🛵 ✈️ 🚀 🚢 ⛵ 🚆 ⌚ 📱 💻 ⌨️ 🖥️ 🖨️ 📷 📹 📺 📻 ⏰ ⏳ 💡 🔦 🔋 🔌 💰 💵
      💳 💎 🔧 🔨 🛠️ 🔑 🔒 🔓 📌 📎 ✂️ 📝 ✏️ 📚 📖 📅 📦 📧 📨 🗑️ 🛒 👕 👗
      👟 👜 🎒 👓 🕶️ 💍 💄 ☂️
    `),
  },
];

/**
 * Emoji yang terakhir dipakai, per perangkat.
 *
 * Disimpan di localStorage, bukan di server: ini kenyamanan kecil yang boleh
 * hilang kapan saja — di jendela privat, setelah data situs dibersihkan —
 * tanpa ada yang rusak. Setiap akses dibungkus try karena penyimpanannya
 * sendiri bisa menolak dibaca.
 */
const RECENT_KEY = 'emoji-terakhir';
const RECENT_MAX = 24;

export function recentEmoji(): string[] {
  try {
    const raw = JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]') as unknown;
    return Array.isArray(raw) ? raw.filter((x): x is string => typeof x === 'string') : [];
  } catch {
    return [];
  }
}

export function rememberEmoji(emoji: string) {
  try {
    const next = [emoji, ...recentEmoji().filter(e => e !== emoji)].slice(0, RECENT_MAX);
    localStorage.setItem(RECENT_KEY, JSON.stringify(next));
  } catch {
    // Tidak diingat bukan kegagalan yang perlu diceritakan.
  }
}

/**
 * Reaksi cepat.
 *
 * Sengaja pendek dan tetap: sembilan dari sepuluh reaksi selesai dengan salah
 * satu dari delapan ini. Sisanya ada di papan emoji lengkap — dan server tetap
 * menerima emoji apa pun yang lolos pemeriksaan satu grafem, jadi pilihan ini
 * tidak mengunci apa-apa.
 */
export const QUICK_REACTIONS = ['👍', '❤️', '😂', '🎉', '🙏', '😮', '😢', '🔥'];

