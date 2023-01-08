import { BiPhotoAlbum ***REMOVED*** from "react-icons/bi"
import AlbumHeader from "./AlbumHeader"
import AlbumItem from "./items/AlbumItem"
import {useGetAllAlbumsQuery***REMOVED*** from "../redux/api/albumSlice";

const AlbumSection = () => {
  const { data: albums, error, isLoading ***REMOVED*** = useGetAllAlbumsQuery()
    return (
        <>
            <AlbumHeader headerIcon={BiPhotoAlbum***REMOVED*** iconColor="teal.400">
                Your Albums
            </AlbumHeader>
        ***REMOVED***isLoading && (<p>loading...</p>)***REMOVED***
        ***REMOVED***!isLoading && albums && albums.map((a, id) => (
                <AlbumItem key={id***REMOVED*** id={a.gaId***REMOVED*** name={a.gaName***REMOVED*** coverId={a.coverId***REMOVED*** coverColor={a.coverColor***REMOVED*** />
            ))***REMOVED***
        </>
    )
***REMOVED***

export default AlbumSection