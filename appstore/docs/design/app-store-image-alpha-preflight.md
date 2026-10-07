# Reject App Store image alpha before upload

Apple's screenshot and creative-image specifications forbid alpha channels and
transparency. The CLI currently checks dimensions and encoded image format but
allows alpha PNGs into screenshot-set and Asset Library reservation requests.

Add a scoped image upload reader that retains the existing encoded-format
check and reads PNG chunk headers through the first IDAT. Reject IHDR color
types 4 and 6, including completely opaque pixels stored with an alpha channel,
and any tRNS chunk. Reading only `image.DecodeConfig` is insufficient: Go's PNG
config decoder can stop after IHDR and miss truecolor or grayscale tRNS.
The bounded chunk scan avoids decoding large pixel buffers.

Apply the reader to screenshot command preflight, the already-opened screenshot
source immediately before reservation, and Asset Library image command
preflight. Preserve rootfs file opening, rewinds, output shape, flags, and valid
RGB PNG/JPEG behavior. Leave generic file validation, video uploads, dimension
inspection, and the advisory review-screenshot warnings unchanged.

RED root command tests send alpha and truecolor tRNS fixtures through screenshot
and creative-image uploads and demonstrate requests before rejection. GREEN
requires rejection with an actionable export instruction and zero requests.
Reader tests cover opaque alpha channels, indexed, truecolor, and grayscale
transparency, and accepted RGB PNG/JPEG. Existing successful upload fixtures
must use opaque images, matching the real upload contract. Full repository gates
and review are separate coordinated steps after these focused checks.
