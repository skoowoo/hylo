const imgWrap = document.getElementById("img-wrap");
const img = document.getElementById("img");
const message = document.getElementById("message");
const saveBtn = document.getElementById("save-btn");
const sendBtn = document.getElementById("send-btn");

window.quickCapture.onInit(({ imageDataUrl, boxW, boxH, imgW, imgH }) => {
  imgWrap.style.width = boxW + "px";
  imgWrap.style.height = boxH + "px";
  img.style.width = imgW + "px";
  img.style.height = imgH + "px";
  img.src = imageDataUrl;
  message.focus();
});

function send() {
  window.quickCapture.submit(message.value.trim());
}

function save() {
  window.quickCapture.save();
}

saveBtn.addEventListener("click", save);
sendBtn.addEventListener("click", send);

// No explicit close button — this is a floating panel, so any click that
// isn't on the composer or an action button is "blank space" and dismisses
// it, same as clicking off a popover. The image/header count as blank too;
// there's nothing to do with them besides dismiss.
document.addEventListener("click", (e) => {
  if (e.target.closest("#composer, #save-btn, #send-btn")) return;
  window.quickCapture.cancel();
});

// A click on the image or inside the textarea doesn't activate Vaultr; a
// click on the panel's blank padding/gaps does. The one difference we can
// see from here is that those clicks don't change which element has DOM
// focus (the image isn't focusable at all, and the textarea is already
// focused going in), while a blank-area click blurs the textarea. A
// mousedown's default action is what performs that blur — preventDefault()
// on it (standard technique, same as a toolbar keeping an input focused)
// stops the blur, so nothing about the interaction changes for the native
// layer no matter where on the panel the click lands. `click` handlers
// below still fire normally; only mousedown's default is suppressed.
document.addEventListener("mousedown", (e) => {
  if (e.target !== message) e.preventDefault();
});

message.addEventListener("keydown", (e) => {
  if (e.key === "Escape") {
    e.preventDefault();
    window.quickCapture.cancel();
  } else if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    send();
  } else if (e.key === "s" && (e.metaKey || e.ctrlKey)) {
    e.preventDefault();
    save();
  }
});
