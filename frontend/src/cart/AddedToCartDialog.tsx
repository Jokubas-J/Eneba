import { useEffect, useRef } from 'react'
import { useNavigate } from 'react-router'
import { Button } from '../components/Button.tsx'
import { CartIcon, VerifiedIcon } from '../components/icons.tsx'
import { useAnimatedClose } from '../components/useAnimatedClose.ts'

// "Added to cart" confirmation (design-reference.md §5, screenshot 13): a centred
// panel over the dimmed page with "Continue shopping" and "View cart". Built on
// <dialog> + showModal(), like the screenshot lightbox: focus stays inside, Escape
// closes it.
export function AddedToCartDialog({ itemCount, onClose }: { itemCount: number; onClose: () => void }) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const navigate = useNavigate()
  // It pops in and fades back out (.pop-dialog in index.css).
  // close() at the end (not just removal) returns focus to where it was before.
  const { closing, requestClose } = useAnimatedClose(() => dialogRef.current?.close(), 200)

  useEffect(() => {
    const dialog = dialogRef.current
    // No clean-up that calls close(): closing fires the "close" event, which tells
    // the parent to remove the dialog. In development React mounts every component,
    // unmounts it and mounts it again (StrictMode, to expose effects without proper
    // clean-up), and such a clean-up made the dialog close itself at once. Removing
    // the element from the page on unmount is enough to make it disappear.
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  return (
    <dialog
      ref={dialogRef}
      aria-labelledby="added-title"
      data-closing={closing || undefined}
      onClose={onClose}
      onCancel={(event) => {
        event.preventDefault()
        requestClose()
      }}
      onClick={(event) => {
        if (event.target === event.currentTarget) requestClose()
      }}
      className="pop-dialog m-auto w-[min(470px,calc(100vw-40px))] bg-purple-light p-8 text-center text-white shadow-panel"
    >
      <span className="relative mx-auto block size-16">
        <CartIcon className="size-16" />
        <VerifiedIcon className="absolute -top-1 -right-2 size-7 text-teal" />
      </span>
      <h2 id="added-title" className="mt-4 text-xl font-extrabold">
        Added to cart
      </h2>
      <p className="mt-1 text-sm">
        You have <strong>{itemCount === 1 ? '1 item' : `${itemCount} items`}</strong> in your cart
      </p>
      <div className="mt-6 flex flex-col justify-center gap-3 sm:flex-row">
        <Button autoFocus onClick={requestClose} className="sm:w-[190px]">
          Continue shopping
        </Button>
        <Button
          variant="outline"
          className="sm:w-[190px]"
          onClick={() => {
            onClose()
            navigate('/cart')
          }}
        >
          View cart
        </Button>
      </div>
    </dialog>
  )
}
