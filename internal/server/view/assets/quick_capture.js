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

// Keep the textarea focused so a click on the padding or image does not
// blur the window. The shell treats a window blur as a click outside.
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
