import { SectionLabel } from '@/components/fanti/section-label';

interface PageHeadingProps {
  /** Uppercase gloss line above the title. */
  gloss: string;
  title: string;
}

/** The design's screen heading: tracked gloss over a wenkai display title. */
function PageHeading({ gloss, title }: PageHeadingProps) {
  return (
    <div>
      <SectionLabel>{gloss}</SectionLabel>
      <h1 className="mt-0.5 font-display font-normal text-2xl leading-(--leading-tight)">
        {title}
      </h1>
    </div>
  );
}

export { PageHeading };
