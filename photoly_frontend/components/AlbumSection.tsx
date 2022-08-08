import { BiPhotoAlbum ***REMOVED*** from "react-icons/bi"
import AlbumHeader from "./AlbumHeader"
import { useAlbumList ***REMOVED*** from "./contexts/AlbumContext"
import AlbumItem from "./items/AlbumItem"

const AlbumSection = () => {
    const { albums ***REMOVED*** = useAlbumList()

    return (
        <>
            <AlbumHeader headerIcon={BiPhotoAlbum***REMOVED*** iconColor="teal.400">
                Your Albums
            </AlbumHeader>
            
   ***REMOVED*****REMOVED***albums.map((a, id) => (
                <AlbumItem key={id***REMOVED*** id={a.id***REMOVED*** name={a.name***REMOVED*** coverId={a.coverId***REMOVED*** coverColor={a.coverColor***REMOVED*** />
            ))***REMOVED***
        </>
    )
***REMOVED***

export default AlbumSection