import { AiFillTags } from "react-icons/ai";
import { useTagList } from "./contexts/TagContext";
import TagHeader from "./TagHeader";
import TagItem from "./items/TagItem";
import { useEffect } from "react";

const TagSection = () => {
  const { tags } = useTagList();

  useEffect(() => {}, [tags]);
  return (
    <>
      <TagHeader headerIcon={AiFillTags} iconColor="teal.400">
        Your Tags
      </TagHeader>

      {tags.map((t, ind) => (
        <TagItem tagName={t.name} tagId={t.id} key={ind} />
      ))}
    </>
  );
};

export default TagSection;
