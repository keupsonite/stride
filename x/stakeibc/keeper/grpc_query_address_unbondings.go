package keeper

import (
	"context"
	"fmt"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"

	epochtypes "github.com/Stride-Labs/stride/v18/x/epochs/types"
	"github.com/Stride-Labs/stride/v18/x/stakeibc/types"
)

const nanosecondsInDay = 86400000000000

func (k Keeper) AddressUnbondings(c context.Context, req *types.QueryAddressUnbondings) (*types.QueryAddressUnbondingsResponse, error) {
	// The function queries all the unbondings associated with Stride addresses.
	// This should provide more visiblity into the unbonding process for a user.

	if req == nil || req.Address == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid request")
	}
	ctx := sdk.UnwrapSDKContext(c)

	// The address field can either be a single address or several comma separated
	addresses := strings.Split(req.Address, ",")

	addressUnbondings := []types.AddressUnbonding{}

	// get the relevant day
	dayEpochTracker, found := k.GetEpochTracker(ctx, epochtypes.DAY_EPOCH)
	if !found {
		return nil, sdkerrors.ErrKeyNotFound
	}
	currentDay := dayEpochTracker.EpochNumber

	epochUnbondingRecords := k.RecordsKeeper.GetAllEpochUnbondingRecord(ctx)

	for _, epochUnbondingRecord := range epochUnbondingRecords {
		for _, hostZoneUnbonding := range epochUnbondingRecord.GetHostZoneUnbondings() {
			for _, userRedemptionRecordId := range hostZoneUnbonding.GetUserRedemptionRecords() {
				userRedemptionRecordComponents := strings.Split(userRedemptionRecordId, ".")
				if len(userRedemptionRecordComponents) != 3 {
					k.Logger(ctx).Error(fmt.Sprintf("invalid user redemption record id %s", userRedemptionRecordId))
					continue
				}
				userRedemptionRecordAddress := userRedemptionRecordComponents[2]

				// Check if the userRedemptionRecordAddress is one targeted by the address(es) in the query
				targetAddress := false
				for _, address := range addresses {
					if userRedemptionRecordAddress == strings.TrimSpace(address) {
						targetAddress = true
						break
					}
				}
				if targetAddress {
					userRedemptionRecord, found := k.RecordsKeeper.GetUserRedemptionRecord(ctx, userRedemptionRecordId)
					if !found {
						continue // the record has already been claimed
					}

					// get the anticipated unbonding time
					unbondingTime := hostZoneUnbonding.UnbondingTime
					if unbondingTime == 0 {
						hostZone, found := k.GetHostZone(ctx, hostZoneUnbonding.HostZoneId)
						if !found {
							return nil, sdkerrors.ErrKeyNotFound
						}
						unbondingFrequency := hostZone.GetUnbondingFrequency()
						daysUntilUnbonding := unbondingFrequency - (currentDay % unbondingFrequency)
						unbondingStartTime := dayEpochTracker.NextEpochStartTime + ((daysUntilUnbonding - 1) * nanosecondsInDay)
						unbondingDurationEstimate := (unbondingFrequency - 1) * 7
						unbondingTime = unbondingStartTime + (unbondingDurationEstimate * nanosecondsInDay)
					}
					unbondingTime = unbondingTime + nanosecondsInDay
					unbondingTimeStr := time.Unix(0, int64(unbondingTime)).UTC().String()

					addressUnbonding := types.AddressUnbonding{
						Address:                userRedemptionRecordAddress,
						Receiver:               userRedemptionRecord.Receiver,
						UnbondingEstimatedTime: unbondingTimeStr,
						Amount:                 userRedemptionRecord.NativeTokenAmount,
						Denom:                  userRedemptionRecord.Denom,
						ClaimIsPending:         userRedemptionRecord.ClaimIsPending,
						EpochNumber:            userRedemptionRecord.EpochNumber,
					}
					addressUnbondings = append(addressUnbondings, addressUnbonding)
				}
			}
		}
	}

	return &types.QueryAddressUnbondingsResponse{AddressUnbondings: addressUnbondings}, nil
}
