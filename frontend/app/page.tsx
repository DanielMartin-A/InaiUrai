import Link from "next/link";

export default function Home() {
  return (
    <main style={{ padding: "2rem", maxWidth: 640 }}>
      <h1>InaiUrai</h1>
      <p>Hire AI employees. Describe the outcome. We deliver.</p>
      <ul>
        <li>
          <Link href="/chat">Chat</Link>
        </li>
        <li>
          <Link href="/pricing">Pricing</Link>
        </li>
        <li>
          <Link href="/dashboard">Dashboard</Link>
        </li>
      </ul>
    </main>
  );
}
