import { AiFillTags ***REMOVED*** from "react-icons/ai"
import { useTagList ***REMOVED*** from "./contexts/TagContext"
import TagHeader from "./TagHeader"
import TagItem from "./TagItem"

const TagSection = () => {
    const { tags ***REMOVED*** = useTagList();
    return (
        <>
            <TagHeader headerIcon={AiFillTags***REMOVED*** iconColor="teal.400">
                Your Tags
            </TagHeader>

   ***REMOVED*****REMOVED***tags.map((t, ind) =>
                <TagItem tagName={t.name***REMOVED*** tagId={t.id***REMOVED*** key={ind***REMOVED***></TagItem>
            )***REMOVED***
        </>
    )
***REMOVED***

export default TagSection