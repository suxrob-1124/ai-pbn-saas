import { ReactNode } from 'react';
import { Navbar } from '@/components/Navbar';
import { Footer } from '@/components/Footer';

export default function PublicLayout({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col min-h-screen">
      {/* Увеличили max-w-5xl до max-w-7xl, чтобы карточки и текст красиво растягивались */}
      <div className="w-full max-w-7xl mx-auto py-6 px-4 flex-1 flex flex-col space-y-8">
        <Navbar />

        {/* Старый <header> с текстом SiteGen AI удален */}

        <main className="flex-1 flex flex-col items-center">{children}</main>

        <Footer />
      </div>
    </div>
  );
}
