# Dice are 3D on the GPU; the server decides the result

Dice roll as real 3D objects with physics, rendered with WebGL in a web worker on an offscreen canvas, so the animation stays smooth on modest phones and never blocks the page. The result still comes from the server-seeded roll: once the physics settles, the face materials are swapped so the face showing up is the server's number. This is the approach of the MIT-licensed 3d-dice/dice-box family, which a spike will confirm before we adopt it or a small Three.js equivalent. Without WebGL, with reduced motion, or when a device is too slow, the same dice draw as 2D icons and the result appears at once.

## Consequences

Dice Sets are stored as one texture atlas per die type, built from a preset or from an uploaded image the owner placed on the unwrapped die. Numbers are drawn on their own layer, so any image stays readable.

A roll plays in three beats. First the dice are thrown in from the edge of the screen, tumble and bounce under physics, and settle on the server's faces. Then they glide together to the centre. Last, the total appears large in the centre, with the breakdown beneath it (kept and dropped dice, modifiers). A natural 20 is marked as a critical hit and a natural 1 as a critical miss, each with its own colour and a short accent. The 2D fallback skips the throw and shows the last beat only.
