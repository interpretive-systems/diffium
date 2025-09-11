export default function Home() {
  return (
    <main style={{fontFamily:'ui-sans-serif, system-ui', padding:'2rem'}}>
      <h1 style={{margin:'0 0 1rem 0'}}>Diffium To‑Do</h1>
      <p style={{color:'#666'}}>Next.js app scaffold. We’ll build the to‑do next.</p>
      <section style={{marginTop:'2rem'}}>
        <h2 style={{margin:'0 0 .5rem 0'}}>Sample</h2>
        <ul style={{lineHeight:1.9}}>
          <li>Wire up basic to‑do list</li>
          <li>Add input + add/remove items</li>
          <li>Persist in localStorage</li>
        </ul>
      </section>
    </main>
  )
}

