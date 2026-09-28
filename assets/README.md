# dfman icon

`dfman.png` is the transparent master artwork generated with the built-in
imagegen tool. `dfman-256.png`, `dfman.ico`, and `dfman.icns` are packaged
size/format derivatives. Regenerate them on macOS with ImageMagick installed:

```sh
bash scripts/build-icons.sh
```

macOS uses the ICNS for the notification app and applies a Finder custom icon
to the installed CLI file. Windows embeds ICO resources into both executables
and the installer, and registers the PNG for the notification identity. Linux
installs the PNG in the hicolor theme for desktop notifications; ELF executables
do not embed Explorer-style file icons.

## Generation prompt

Use case: logo-brand. Create a polished square app icon mascot for dfman, a
dotfiles tool. A stylized friendly superhero wearing a T-shirt with one large
contrasting solid circular DOT on the chest, flying diagonally upward, one arm
extended ahead with an OPEN hand (not a clenched fist), other arm back, cape
streaming behind. Bold compact silhouette, clean rounded vector-like
illustration, restrained vibrant colors, strong contrast, minimal internal
detail, attractive professional desktop application icon that remains legible
at 32 pixels. Whole figure visible, centered, fills most of square with
comfortable margin. Transparent background, no tile, no text, no letters, no
watermark, no existing superhero insignia. The chest dot and forward open hand
must be clearly visible.

## Revision prompt

Edited with the built-in imagegen tool using the original icon as reference:

Edit this dfman superhero icon. Change only these two details: (1) the forward
extended open hand must become a naturally CLOSED CLENCHED FIST, superhero
flying pose, correct thumb curled over fingers. (2) Remove the single large
blue dot on the white T-shirt and replace it with a simple blue two-row list
emblem: each row is a small solid circular dot followed by a solid horizontal
rounded bar, like '. -------' on the first row and '. -------' on the second
row. Exactly two dots and two bars, no filenames, no letters, no text. Fit this
emblem to the shirt's perspective and keep it bold and readable at icon size.
Preserve character face, hair, blue cape, colors, lighting, flying pose, white
shirt, illustration style, framing, and genuine transparent background.

## Three-row revision prompt

Edited with the built-in imagegen tool:

Edit only the blue emblem on the superhero's white T-shirt: change the current
TWO dot-and-line rows to exactly THREE evenly spaced dot-and-line rows. Each row
has one small solid blue circular dot followed by one solid blue horizontal
rounded bar. Exactly three dots and three bars total, no filenames, letters, or
text. Center the three-row emblem on the shirt and follow its perspective. Keep
everything else unchanged: closed forward fist, face, hair, body, flying pose,
blue cape, colors, rendering, framing, and genuinely transparent background.
